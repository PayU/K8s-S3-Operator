package aws

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	"github.com/PayU/K8s-S3-Operator/config"

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

func (a *AwsClient) BucketExists(name string) (bool, error) {
	_, err := a.s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(name)})
	if err != nil {
		if awsErr, isAwsErr := err.(awserr.Error); isAwsErr && awsErr.Code() == s3.ErrCodeNoSuchBucket {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func (a *AwsClient) validateBucketName(name string) bool {
	if len(name) > 4 && name[:4] == "xn--" {
		return false
	}
	if len(name) > 8 && name[len(name)-8:] == "-s3alias" {
		return false
	}
	match, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9\\-]+[a-zA-Z0-9]$", name)
	return match
}

func (a *AwsClient) HandleBucketCreation(bucketSpec *s3operatorv1.S3BucketSpec, resourceName string) (bool, error) {
	a.Log.Info("HandleBucketCreation")
	if !a.validateBucketName(bucketSpec.BucketName) {
		validateErr := errors.New("error bucket name is unvalid")
		a.Log.Error(validateErr, "bucket name is unvalid")
		return false, validateErr
	}
	BucketExist, err := a.BucketExists(bucketSpec.BucketName)
	if BucketExist {
		a.Log.Error(err, "bucket allredy exsist")
		return false, errors.New("bucket allredy exsist")
	}
	bucketInput := a.CreateBucketInput(bucketSpec.BucketName, config.Region())
	res, err := a.CreateBucket(*bucketInput)
	if err != nil {
		a.Log.Error(err, "got error in create bucket function")
		return false, err
	}
	a.PutBucketTagging(bucketSpec.BucketName, &bucketSpec.Tags)
	roleName := getRoleName(bucketSpec.BucketName)
	tag := config.DefaultTag()
	a.iamClient.CreateIamRole(roleName, &iam.Tag{Key: tag.Key, Value: tag.Value})

	a.PutBucketPolicy(bucketSpec.BucketName, roleName)
	if bucketSpec.Encryption {
		a.PutBucketEncrypt(bucketSpec.BucketName)
	}
	a.Log.Info("succeded to create new bucket", "createBucketOutput", res)
	return true, nil

}

func (a *AwsClient) HandleBucketDeletion(bucketsK8S []s3operatorv1.S3Bucket) (bool, error) {
	a.Log.Info("HandleBucketDeletion function")
	deleteFlag := false
	bukcetsFromAws, err := a.getAllBucketsByTag(config.DefaultTag())
	if err != nil {
		a.Log.Error(err, "error in HandleBucketDeletion in getAllBucketsByTag")
		return false, err
	}
	a.Log.Info("got buckets from getAllBucketsByTag", "bucket", bukcetsFromAws)
	mapResourceK8s := make(map[string]struct{}, len(bucketsK8S))
	for _, b := range bucketsK8S {
		mapResourceK8s[b.Spec.BucketName] = struct{}{}
	}
	for _, bucketName := range bukcetsFromAws {
		if _, found := mapResourceK8s[*bucketName]; !found {
			err := a.CleanupsBucket(*bucketName)
			if err != nil {
				a.Log.Error(err, "error in CleanupsBucket in bucket: "+*bucketName)
				return false, err
			}
			res, err := a.DeleteBucket(*bucketName)
			if err != nil {
				a.Log.Error(err, "err delete bucket"+*bucketName)
				return false, err
			}
			a.iamClient.DeleteIamRole(getRoleName(*bucketName))
			a.Log.Info("succeded to delete bucket", "res", res)
			deleteFlag = true
		}
	}
	return deleteFlag, nil
}

func (a *AwsClient) CreateBucket(bucketInput s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	a.Log.Info("CreateBucket function")
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
			a.Log.Error(err, "error in creatBucket function")
			return nil, err
		}
	}
	a.Log.Info("result from bucket creation", "result", res)
	return res, nil

}

func (a *AwsClient) PutBucketTagging(bucketName string, bucketTags *map[string]string) (bool, error) {
	var tags []*s3.Tag
	for key, val := range *bucketTags {
		tag := s3.Tag{
			Key:   &key,
			Value: &val,
		}
		tags = append(tags, &tag)
	}
	tags = append(tags, config.DefaultTag())

	input := &s3.PutBucketTaggingInput{
		Bucket:  aws.String(bucketName),
		Tagging: &s3.Tagging{TagSet: tags},
	}
	res, err := a.s3Client.PutBucketTagging(input)
	if err != nil {
		a.Log.Error(err, "error PutBucketTagging")
		return false, err
	}
	a.Log.Info("succeded to PutBucketTagging", "res", res, "input", input)
	return true, nil
}

func (a *AwsClient) CreateBucketInput(bucketName string, bucketRegion string) *s3.CreateBucketInput {
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	s3Input.CreateBucketConfiguration = &s3.CreateBucketConfiguration{LocationConstraint: aws.String(bucketRegion)}
	a.Log.Info("bucketInput", "bucketInput", s3Input)
	return s3Input
}

func (a *AwsClient) DeleteBucket(bucketName string) (*s3.DeleteBucketOutput, error) {
	a.Log.Info("DeleteBucket function")
	res, err := a.s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	return res, err
}
func (a *AwsClient) CleanupsBucket(bucketName string) error {
	iter := s3manager.NewDeleteListIterator(a.s3Client, &s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
	})
	if err := s3manager.NewBatchDeleteWithClient(a.s3Client).Delete(aws.BackgroundContext(), iter); err != nil {
		a.Log.Error(err, "Unable to delete objects from bucket %q", bucketName)
		return err
	}
	a.Log.Info("succeded to cleanup bucket: " + bucketName)
	return nil
}

func (a *AwsClient) PutBucketPolicy(bucketName string, roleName string) (*s3.PutBucketPolicyOutput, error) {
	a.Log.Info("PutBucketPolicy fuction")

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
		a.Log.Error(err, "error in PutBucketPolicy in Marshal")
		return nil, err

	}

	input := &s3.PutBucketPolicyInput{
		Bucket: &bucketName,
		Policy: aws.String(string(bucketPolicy)),
	}

	res, err := a.s3Client.PutBucketPolicy(input)
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
	res, err := a.s3Client.PutBucketEncryption(input)
	if err != nil {
		a.Log.Error(err, "not succsede to PutBucketEncrypt")
		return false, err
	}
	a.Log.Info("succeded to encrypt bucket", "res", res)
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
				a.Log.Info("in for loop on page", "b", b)
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

func CreateSession() *session.Session {
	awsConfig := &aws.Config{
		Region:                        aws.String(config.Region()),
		S3ForcePathStyle:              aws.Bool(config.AwsS3ForcePathStyle()),
		Endpoint:                      aws.String(config.AwsEndpoint()),
		CredentialsChainVerboseErrors: aws.Bool(config.AwsCredentialsChainVerboseErrors()),
		DisableSSL:                    aws.Bool(config.AwsConfigDisableSSL()),
	}
	awsConfig.HTTPClient = &http.Client{Timeout: config.Timeout()}

	ses := session.Must(session.NewSession(awsConfig))

	return ses
}

func setS3Client(Log *logr.Logger, ses *session.Session) *s3.S3 {
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
	ses := CreateSession()
	if ses == nil {
		err := errors.New("ses is nil")
		Log.Error(err, "error in create new session")
	} else {
		Log.Info("session", "ses", ses)
		return setS3Client(Log, ses), setRGTAClient(Log, ses), setIamClient(Log, ses)
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
