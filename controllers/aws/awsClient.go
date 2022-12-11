package aws

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	"github.com/PayU/K8s-S3-Operator/controllers/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type AwsClient struct {
	s3Client   *s3.S3
	RGTAClient *resourcegroupstaggingapi.ResourceGroupsTaggingAPI
	Log        *logr.Logger
	iamClient  *IamClient
}

func (a *AwsClient) BucketExists(name string, log *logr.Logger) (bool, error) {
	a.Log = log
	_, err := a.s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(name)})
	if err != nil {
		if awsErr, isAwsErr := err.(awserr.Error); isAwsErr && awsErr.Code() == s3.ErrCodeNoSuchBucket {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func (a *AwsClient) validateBucketName(name string) (bool, error) {
	if len(name) > 4 && name[:4] == "xn--" {
		return false, errors.New("bucket name can't start with xn--")
	}
	if len(name) > 8 && name[len(name)-8:] == "-s3alias" {
		return false, errors.New("bucket name can't end with -s3alias")
	}
	match, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9\\-]+[a-zA-Z0-9]$", name)
	if !match {

		return match, errors.New("bucket name not mutch pattern: '^[a-zA-Z][a-zA-Z0-9\\-]+[a-zA-Z0-9]$' ")
	}
	return match, nil
}

func (a *AwsClient) HandleBucketCreation(bucketSpec *s3operatorv1.S3BucketSpec, bucketName string, log *logr.Logger) (bool, error) {
	a.Log = log
	validName, err := a.validateBucketName(bucketName)
	if !validName {
		a.Log.Error(err, "bucket name is unvalid")
		return false, err
	}
	BucketExist, err := a.BucketExists(bucketName, a.Log)
	if BucketExist {
		a.Log.Error(err, "bucket allredy exsist")
		return false, errors.New("bucket allredy exsist")
	}
	bucketInput := a.CreateBucketInput(bucketName, config.Region())
	_, err = a.CreateBucket(*bucketInput)
	if err != nil {
		a.Log.Error(err, "got error in create bucket function")
		return false, err
	}
	a.PutBucketTagging(bucketName, &bucketSpec.Tags)
	roleName := getRoleName(bucketName)
	tag := config.DefaultTag()
	a.iamClient.CreateIamRole(roleName, &iam.Tag{Key: tag.Key, Value: tag.Value}, a.Log)

	a.PutBucketPolicy(bucketName, roleName)
	if bucketSpec.Encryption {
		a.PutBucketEncrypt(bucketName)
	}
	a.Log.Info("S3 bucket creation process finished successfully", "region", config.Region())
	return true, nil

}

func (a *AwsClient) HandleBucketDeletion(bucketToDelete string, log *logr.Logger) (bool, error) {
	a.Log = log
	a.Log.Info(" Start to delete s3 bucket from aws")
	isBucketExists, err := a.BucketExists(bucketToDelete, a.Log)
	if isBucketExists {
		_, err := a.DeleteBucket(bucketToDelete)
		if err != nil {
			a.Log.Error(err, "err delete bucket")
			return false, err
		}
		_, err = a.iamClient.DeleteIamRole(getRoleName(bucketToDelete), a.Log)
		a.Log.Info("s3 bucket deletion from aws finished successfully")
	}
	return true, err
}

func (a *AwsClient) HandleBucketUpdate(bucketName string, bucketSpec *s3operatorv1.S3BucketSpec, log *logr.Logger) (bool, error) {
	a.Log = log
	a.Log.Info("HandleBucketUpdate function")
	res, err := a.UpdateBucketTags(bucketName, bucketSpec.Tags)

	a.Log.Info("finish to HandleBucketUpdate")
	return res, err
}

func (a *AwsClient) UpdateBucketTags(bucketName string, tagsToUpdate map[string]string) (bool, error) {
	a.Log.V(1).Info("UpdateBucketTags function")
	if tagsToUpdate == nil {
		tagsToUpdate = map[string]string{}
	}
	tagsFromAws, err := a.s3Client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: aws.String(bucketName)})
	if err != nil {
		a.Log.Error(err, "error from GetBucketTagging")
		return false, err
	}
	isDiffTags, diffTags := a.FindIfDiffTags(tagsToUpdate, tagsFromAws.TagSet)
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
func (a *AwsClient) FindIfDiffTags(tagsToUpdate map[string]string, tagsFromAws []*s3.Tag) (bool, []*s3.Tag) {
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

func (a *AwsClient) CreateBucket(bucketInput s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
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

func (a *AwsClient) PutBucketTagging(bucketName string, bucketTags *map[string]string) (bool, error) {
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

func (a *AwsClient) CreateBucketInput(bucketName string, bucketRegion string) *s3.CreateBucketInput {
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	s3Input.CreateBucketConfiguration = &s3.CreateBucketConfiguration{LocationConstraint: aws.String(bucketRegion)}
	return s3Input
}

func (a *AwsClient) DeleteBucket(bucketName string) (*s3.DeleteBucketOutput, error) {
	a.Log.Info("DeleteBucket function")
	res, err := a.s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	return res, err
}
func (a *AwsClient) CleanupsBucket(bucketName string) error {
	a.Log.Info("CleanupsBucket function")

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

func (a *AwsClient) PutBucketPolicy(bucketName string, roleName string) (*s3.PutBucketPolicyOutput, error) {
	a.Log.Info("adding bucket policy for s3 bucket", "roleName:", roleName)

	// Create a policy using map interface. Filling in the bucket as the
	// resource.
	AllPremisionToRole := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Sid":    "AllPremisionToRole" + bucketName,
				"Effect": "Allow",
				"Principal": []string{
					"AWS: arn:aws:iam:::role/" + roleName,
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
		a.Log.Error(err, "error in PutBucketPolicy in Marshal", "roleName:", roleName)
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

func (a *AwsClient) PutBucketEncrypt(bucketName string) (bool, error) {
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

func (a *AwsClient) DeleteBucketPolicy(bucketName string) (*s3.DeleteBucketPolicyOutput, error) {
	a.Log.Info("DeleteBucketPolicy function")
	input := &s3.DeleteBucketPolicyInput{
		Bucket: &bucketName,
	}
	res, err := a.s3Client.DeleteBucketPolicy(input)
	if err != nil {
		a.Log.Error(err, "error in DeleteBucketPolicy")
	}
	return res, err
}

func (a *AwsClient) getAllBucketsByTag(filterTag *s3.Tag) ([]*string, error) {
	a.Log.Info("getAllBucketsByTag function", "filterTag", filterTag)
	var buckets []*string

	input := resourcegroupstaggingapi.GetResourcesInput{
		ResourceARNList:  []*string{aws.String("arn:aws:s3")},
		ResourcesPerPage: config.ResourcesPerPage(),
		TagFilters:       []*resourcegroupstaggingapi.TagFilter{{Key: filterTag.Key, Values: []*string{filterTag.Value}}},
	}
	err := a.RGTAClient.GetResourcesPages(&input,
		func(page *resourcegroupstaggingapi.GetResourcesOutput, isLastPage bool) bool {
			a.Log.Info("in GetResourcesPages ", "page", page)
			for _, b := range page.ResourceTagMappingList {
				bucketSplitArrayARN := strings.Split(*b.ResourceARN, ":")
				buckets = append(buckets, &bucketSplitArrayARN[len(bucketSplitArrayARN)-1])
			}
			return isLastPage
		})
	if err != nil {
		a.Log.Error(err, "error in GetResourcesPages")
		return nil, err
	}
	return buckets, nil
}

func getRoleName(bucketName string) string {
	roleName := bucketName + "IAM-ROLE-S3Operator"
	return roleName
}

func CreateSession(Log *logr.Logger) *session.Session {
	awsConfig := &aws.Config{
		Region:                        aws.String(config.Region()),
		S3ForcePathStyle:              aws.Bool(config.AwsS3ForcePathStyle()),
		Endpoint:                      aws.String(config.AwsEndpoint()),
		CredentialsChainVerboseErrors: aws.Bool(config.AwsCredentialsChainVerboseErrors()),
		DisableSSL:                    aws.Bool(config.AwsConfigDisableSSL()),
	}
	awsConfig.HTTPClient = &http.Client{Timeout: config.Timeout()}
	Log.Info("Create Session with aws config ", "Region", awsConfig.Region, "Endpoint", awsConfig.Endpoint)
	ses := session.Must(session.NewSession(awsConfig))

	return ses
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

func setRGTAClient(Log *logr.Logger, ses *session.Session) *resourcegroupstaggingapi.ResourceGroupsTaggingAPI {
	Log.Info("create RGTAClient wit session", "session", *ses)
	RGTAClient := resourcegroupstaggingapi.New(ses)
	if RGTAClient == nil {
		RGTAClientErr := errors.New("error in create RGTAClient")
		Log.Error(RGTAClientErr, "didnt succeded to create s3Client")
	} else {
		Log.Info(" succeded create RGTAClient", "client", *RGTAClient)
	}
	return RGTAClient
}

func setClients(Log *logr.Logger) (*s3.S3, *resourcegroupstaggingapi.ResourceGroupsTaggingAPI, *iam.IAM) {
	ses := CreateSession(Log)
	if ses == nil {
		err := errors.New("ses is nil")
		Log.Error(err, "error in create new session")
	} else {
		Log.Info("session", "ses", ses)
		return SetS3Client(Log, ses), setRGTAClient(Log, ses), setIamClient(Log, ses)
	}
	return &s3.S3{}, &resourcegroupstaggingapi.ResourceGroupsTaggingAPI{}, &iam.IAM{}
}

func GetAwsClient(logger *logr.Logger) *AwsClient {
	s3Client, rgtaClient, iamClient := setClients(logger)
	return &AwsClient{
		s3Client:   s3Client,
		Log:        logger,
		RGTAClient: rgtaClient,
		iamClient:  &IamClient{IamClient: iamClient, Log: logger},
	}
}
