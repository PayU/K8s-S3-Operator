package aws

import (
	"encoding/json"
	"errors"
	"regexp"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	"github.com/PayU/K8s-S3-Operator/controllers/config"
	"github.com/go-logr/logr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func (a *AwsClient) ValidateBucketName(name string) error {
	if len(name) > 4 && name[:4] == "xn--" {
		return errors.New("bucket name can't start with xn--")
	}
	if len(name) > 8 && name[len(name)-8:] == "-s3alias" {
		return errors.New("bucket name can't end with -s3alias")
	}
	match, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9\\-]+[a-zA-Z0-9]$", name)
	if !match {

		return errors.New("bucket name not mutch pattern: '^[a-zA-Z][a-zA-Z0-9\\-]+[a-zA-Z0-9]$' ")
	}
	return nil
}

func (a *AwsClient) HandleBucketCreation(bucketSpec *s3operatorv1.S3BucketSpec, bucketName string, namespace string) error {

	bucketInput := a.createBucketInput(bucketName, config.Region())
	_, err := a.createBucket(*bucketInput)
	if err != nil {
		a.Log.Error(err, "got error in create bucket function")
		return err
	}
	a.putBucketTagging(bucketName, &bucketSpec.Tags)
	roleName := GetRoleName(bucketName)
	tag := config.DefaultTag()
	a.iamClient.createIamRole(roleName, &iam.Tag{Key: tag.Key, Value: tag.Value}, a.Log)

	a.putBucketPolicy(bucketName, roleName)
	if bucketSpec.Encryption {
		a.putBucketEncrypt(bucketName)
	}
	a.Log.Info("S3 bucket creation process finished successfully", "region", config.Region())
	return nil

}

func (a *AwsClient) HandleBucketDeletion(bucketToDelete string) (bool, error) {
	a.Log.Info(" Start to delete s3 bucket from aws")
	isBucketExists, err := a.IsBucketExists(bucketToDelete)
	if isBucketExists {
		isOwner, err := a.isBucketManagedByOperator(bucketToDelete)
		if !isOwner {
			return false, err
		}
		err = a.cleanupsBucketContent(bucketToDelete)
		if err != nil {
			a.Log.Error(err, "err to cleanup bucket")
			return false, err
		}
		_, err = a.deleteBucket(bucketToDelete)
		if err != nil {
			a.Log.Error(err, "err delete bucket")
			return false, err
		}
		_, err = a.iamClient.deleteIamRole(GetRoleName(bucketToDelete), a.Log)
		a.Log.Info("s3 bucket deletion from aws finished successfully")
	}
	return true, err
}

func (a *AwsClient) HandleBucketUpdate(bucketName string, bucketSpec *s3operatorv1.S3BucketSpec) error {
	a.Log.V(1).Info("HandleBucketUpdate function")
	isOwner, err := a.isBucketManagedByOperator(bucketName)
	if isOwner {
		_, err = a.updateBucketTags(bucketName, bucketSpec.Tags)
	} else {
		if err == nil {
			err = errors.New("cant update bucket that not manage by operator")
		}
	}
	a.Log.Info("finish to HandleBucketUpdate")
	return err
}

func (a *AwsClient) IsBucketExists(name string) (bool, error) {
	_, err := a.s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(name)})
	if err != nil {
		if awsErr, isAwsErr := err.(awserr.Error); isAwsErr && awsErr.Code() == s3.ErrCodeNoSuchBucket {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *AwsClient) updateBucketTags(bucketName string, tagsToUpdate map[string]string) (bool, error) {
	a.Log.V(1).Info("UpdateBucketTags function")
	if tagsToUpdate == nil {
		tagsToUpdate = map[string]string{}
	}
	tagsFromAws, err := a.s3Client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: aws.String(bucketName)})
	if err != nil {
		a.Log.Error(err, "error from GetBucketTagging")
		return false, err
	}
	isDiffTags, diffTags := a.findIfDiffTags(tagsToUpdate, tagsFromAws.TagSet)
	if isDiffTags {
		_, err := a.s3Client.PutBucketTagging(&s3.PutBucketTaggingInput{Bucket: &bucketName, Tagging: &s3.Tagging{TagSet: diffTags}})
		if err != nil {
			a.Log.Error(err, "error from PutBucketTagging")
		} else {
			a.Log.Info("finish to update tags")

		}
	} else {
		a.Log.Info("no tags to update")
	}
	return true, nil
}
func (a *AwsClient) findIfDiffTags(tagsToUpdate map[string]string, tagsFromAws []*s3.Tag) (bool, []*s3.Tag) {
	a.Log.V(1).Info("FindDiffTags function")
	isDiffTags := false
	newTags := []*s3.Tag{}
	mapAwsTags := map[string]string{}

	for _, tag := range tagsFromAws {
		tagToCheck := *tag
		mapAwsTags[*tag.Key] = *tag.Value
		if len(*tagToCheck.Key) < len(config.TagPrefix()) || (*tagToCheck.Key)[:len(config.TagPrefix())] != config.TagPrefix() {
			newTags = append(newTags, &tagToCheck) //add all tags that dont have the operator prefix
			a.Log.Info("add tag from aws", "tag", tagToCheck)

		} else { //all the tags from aws that have the Tag prefix
			tagKeyWithoutPrefix := (*tagToCheck.Key)[:len(config.TagPrefix())]
			val, ok := tagsToUpdate[tagKeyWithoutPrefix]
			if !ok || val != *tagToCheck.Value {
				isDiffTags = true
				a.Log.Info("found tags to update", "tagsToUpdate", tagToCheck)
			}
		}
	}
	for key, val := range tagsToUpdate { //add all the tags from resource to the tags array
		Tagkey := config.TagPrefix() + key
		Tagval := val
		tag := s3.Tag{Key: &Tagkey, Value: &Tagval}
		a.Log.V(1).Info("add tag from spec", "tag", tag)
		newTags = append(newTags, &tag)
		val, ok := mapAwsTags[Tagkey]
		if !ok || val != Tagval {
			isDiffTags = true
			a.Log.Info("found tags to update", "tagsToUpdate", tag)
		}

	}
	a.Log.V(1).Info("returend from find diff Tags", "isDiffTags", isDiffTags, "newTags", newTags)
	return isDiffTags, newTags
}
func (a *AwsClient) isBucketManagedByOperator(bucketName string) (bool, error) {
	tagsFromAws, err := a.s3Client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: aws.String(bucketName)})
	if err != nil {
		a.Log.Error(err, "error from GetBucketTagging in checkIfOwnerBucketByTag")
		return false, err
	}
	defaultTag := config.DefaultTag()
	for _, tag := range tagsFromAws.TagSet {
		if defaultTag.GoString() == tag.GoString() {
			return true, nil
		}
	}
	a.Log.Info("bucket is not manage by the operator")
	return false, nil

}

func (a *AwsClient) createBucket(bucketInput s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	a.Log.Info("Starting to create S3 bucket on AWS", "region", *bucketInput.CreateBucketConfiguration.LocationConstraint)
	res, err := a.s3Client.CreateBucket(&bucketInput)
	if err != nil { //  cast err to awserr.Error to get the Code and
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				a.Log.Error(aerr, s3.ErrCodeBucketAlreadyExists)
				return nil, aerr
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				a.Log.Error(aerr, s3.ErrCodeBucketAlreadyOwnedByYou)
				return nil, aerr
			default:
				a.Log.Error(aerr, aerr.Error())
				return nil, aerr
			}
		} else {
			// Message from an error.
			a.Log.Error(err, "error in creatBucket function", "region", *bucketInput.CreateBucketConfiguration.LocationConstraint)
			return nil, err
		}
	}
	a.Log.Info("S3 bucket creation finished successfully", "region", *bucketInput.CreateBucketConfiguration.LocationConstraint)
	return res, nil

}

func (a *AwsClient) putBucketTagging(bucketName string, bucketTags *map[string]string) (bool, error) {
	tags := make([]*s3.Tag, 0)
	for key, val := range *bucketTags {
		Tagkey := config.TagPrefix() + key
		Tagval := val
		tag := s3.Tag{
			Key:   &Tagkey,
			Value: &Tagval,
		}
		tags = append(tags, &tag)
	}
	tags = append(tags, config.DefaultTag())

	input := &s3.PutBucketTaggingInput{
		Bucket:  aws.String(bucketName),
		Tagging: &s3.Tagging{TagSet: tags},
	}
	a.Log.Info("Adding Tags to s3 bucket", "bucket_tags", *input.Tagging)
	_, err := a.s3Client.PutBucketTagging(input)
	if err != nil {
		a.Log.Error(err, "error PutBucketTagging")
		return false, err
	}
	a.Log.Info("S3 bucket tagging finished successfully", "bucket_tags", *input.Tagging)
	return true, nil
}

func (a *AwsClient) createBucketInput(bucketName string, bucketRegion string) *s3.CreateBucketInput {
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	s3Input.CreateBucketConfiguration = &s3.CreateBucketConfiguration{LocationConstraint: aws.String(bucketRegion)}
	return s3Input
}

func (a *AwsClient) deleteBucket(bucketName string) (*s3.DeleteBucketOutput, error) {
	a.Log.Info("DeleteBucket function")
	res, err := a.s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	return res, err
}

// cleanupsBucketContent function - delete all the object that inside the bucket (required for deleting bucket)
func (a *AwsClient) cleanupsBucketContent(bucketName string) error {
	a.Log.V(1).Info("CleanupsBucket function")

	iter := s3manager.NewDeleteListIterator(a.s3Client, &s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
	})
	if err := s3manager.NewBatchDeleteWithClient(a.s3Client).Delete(aws.BackgroundContext(), iter); err != nil {
		a.Log.Error(err, "Unable to delete objects from bucket")
		return err
	}
	a.Log.Info("succeded to cleanup bucket")
	return nil
}

func (a *AwsClient) putBucketPolicy(bucketName string, iamRole string) (*s3.PutBucketPolicyOutput, error) {
	a.Log.Info("adding bucket policy for s3 bucket", "iamRole:", iamRole)

	// Create a policy using map interface. Filling in the bucket as the
	// resource.
	AllPremisionToRole := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Sid":    "AllPremisionToRole" + bucketName,
				"Effect": "Allow",
				"Principal": []string{
					"AWS: " + iamRole,
				},
				"Action": []string{
					"s3:*",
				},
				"Resource": []string{
					"arn:aws:s3:::" + bucketName,
					"arn:aws:s3:::" + bucketName + "/*",
				},
			},
		},
	}
	bucketPolicy, err := json.Marshal(AllPremisionToRole)
	if err != nil {
		a.Log.Error(err, "error in PutBucketPolicy in Marshal", "iamRole:", iamRole)
		return nil, err

	}

	input := &s3.PutBucketPolicyInput{
		Bucket: &bucketName,
		Policy: aws.String(string(bucketPolicy)),
	}
	res, err := a.s3Client.PutBucketPolicy(input)
	if err != nil {
		a.Log.Error(err, "error in put bucket policy", bucketName, "policy:", *input.Policy)
	} else {
		a.Log.Info("Attach bucket policy to s3 bucket finished successfully", "policy:", *input.Policy)
	}
	return res, err
}

func (a *AwsClient) putBucketEncrypt(bucketName string) (bool, error) {
	a.Log.Info("PutBucketEncrypt function")
	encryptRules := []*s3.ServerSideEncryptionRule{{
		BucketKeyEnabled:                   aws.Bool(true),
		ApplyServerSideEncryptionByDefault: &s3.ServerSideEncryptionByDefault{SSEAlgorithm: aws.String("AES256")}},
	}
	sSEncryptConfiguration := s3.ServerSideEncryptionConfiguration{
		Rules: encryptRules,
	}

	input := &s3.PutBucketEncryptionInput{
		Bucket:                            &bucketName,
		ServerSideEncryptionConfiguration: &sSEncryptConfiguration,
	}
	a.Log.Info("PutBucketEncryption input", "ServerSideEncryptionConfiguration:", *input.ServerSideEncryptionConfiguration)
	_, err := a.s3Client.PutBucketEncryption(input)
	if err != nil {
		a.Log.Error(err, "not succsede to PutBucketEncrypt")
		return false, err
	}
	a.Log.Info("succeded to encrypt bucket")
	return true, nil

}
func SetS3Client(Log *logr.Logger, ses *session.Session) *s3.S3 {
	Log.Info("create s3Client wit session", "session", *ses)
	s3Client := s3.New(ses)
	if s3Client == nil {
		s3ClientErr := errors.New("error in create s3 client")
		Log.Error(s3ClientErr, "didnt succeded to create s3Client")
	} else {
		Log.Info(" succeded create s3Client", "client", *s3Client)
	}
	return s3Client
}
