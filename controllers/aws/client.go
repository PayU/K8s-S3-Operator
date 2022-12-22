package aws

import (
	"errors"
	"net/http"

	"github.com/PayU/K8s-S3-Operator/controllers/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
)

type AwsClient struct {
	s3Client  *s3.S3
	Log       *logr.Logger
	iamClient *IamClient
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

func setClients(Log *logr.Logger) (*s3.S3, *iam.IAM) {
	ses := CreateSession(Log)
	if ses == nil {
		err := errors.New("ses is nil")
		Log.Error(err, "error in create new session")
	} else {
		Log.Info("session", "ses", ses)
		return SetS3Client(Log, ses), setIamClient(Log, ses)
	}
	return &s3.S3{}, &iam.IAM{}
}

func GetAwsClient(logger *logr.Logger, c client.Client) *AwsClient {
	s3Client, iamClient := setClients(logger)
	return &AwsClient{
		s3Client:  s3Client,
		Log:       logger,
		iamClient: &IamClient{IamClient: iamClient, Log: logger},
	}
}
