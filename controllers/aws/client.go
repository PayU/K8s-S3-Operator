package aws

import (
	"errors"
	"net/http"
	"regexp"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	"github.com/PayU/K8s-S3-Operator/controllers/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/s3"
)

type AwsClient struct {
	s3Client   *s3.S3
	RGTAClient *resourcegroupstaggingapi.ResourceGroupsTaggingAPI
	Log        *logr.Logger
	iamClient  *IamClient
}


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
		err := a.cleanupsBucketContent(bucketToDelete)
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
	_, err := a.updateBucketTags(bucketName, bucketSpec.Tags)

	a.Log.Info("finish to HandleBucketUpdate")
	return err
}



func GetRoleName(bucketName string) string {
	roleName := bucketName + "IAM-ROLE-S3Operator"
	iamRole := "arn:aws:iam:::role/" + roleName
	return iamRole
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

func GetAwsClient(logger *logr.Logger, c client.Client) *AwsClient {
	s3Client, rgtaClient, iamClient := setClients(logger)
	return &AwsClient{
		s3Client:   s3Client,
		Log:        logger,
		RGTAClient: rgtaClient,
		iamClient:  &IamClient{IamClient: iamClient, Log: logger},
	}
}
