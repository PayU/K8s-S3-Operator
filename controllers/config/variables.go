package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

var awsEndpoint string
var region string
var awsConfigDisableSSL bool
var timeout int
var resourcePerPage int64 = 100
var awsCredentialsChainVerboseErrors bool
var awsS3ForcePathStyle bool
var devMode bool

func init() {
	var err error
	if region = os.Getenv("REGION"); region == "" {
		region = "eu-central-1"
	}
	if awsEndpoint = os.Getenv("AWS_ENDPOINT"); awsEndpoint == "" {
		awsEndpoint = "http://localstack.k8s-s3-operator-system:4566"
	}
	if os.Getenv("AWS_CONFIG_DISABLE_SSL") == "" {
		awsConfigDisableSSL = true
	} else {
		awsConfigDisableSSL = os.Getenv("AWS_CONFIG_DISABLE_SSL") != "false"
	}
	if strTime := os.Getenv("Timeout"); strTime != "" {
		if timeout, err = strconv.Atoi(strTime); err != nil {
			panic(fmt.Sprintf("error on parsing timeout:[%v]", err))
		} else {
			timeout = 5
		}
	}
	if strResourcePerPage := os.Getenv("RESOURCE_PER_PAGE"); strResourcePerPage != "" {
		if resourcePerPage, err = strconv.ParseInt(strResourcePerPage, 10, 64); err != nil {
			panic(fmt.Sprintf("error on parsing resourcePerPage:[%v]", err))
		} else {
			resourcePerPage = 100
		}
	}
	if os.Getenv("AWS_CREDENTIALS_CHAIN_VERBOSE_ERRORS") == "" {
		awsCredentialsChainVerboseErrors = true
	} else {
		awsCredentialsChainVerboseErrors = os.Getenv("AWS_CREDENTIALS_CHAIN_VERBOSE_ERRORS") != "false"
	}
	if os.Getenv("AWS_S3_FORCE_PATH_STYLE") == "" {
		awsS3ForcePathStyle = true
	} else {
		awsS3ForcePathStyle = os.Getenv("AWS_S3_FORCE_PATH_STYLE") != "false"
	}
	if os.Getenv("DEVMODE") == "true" {
		devMode = true
	} else {
		devMode = false
	}
}

func Timeout() time.Duration {
	return time.Duration(time.Duration(timeout) * time.Second)
}
func Region() string {
	return region
}
func AwsEndpoint() string {
	return awsEndpoint
}
func AwsConfigDisableSSL() bool {
	return awsConfigDisableSSL
}
func DefaultTag() *s3.Tag {
	DEFAULT_TAG := &s3.Tag{Key: aws.String("createdBy"), Value: aws.String("s3Operator")}
	return DEFAULT_TAG

}
func ResourcesPerPage() *int64 {
	return aws.Int64(resourcePerPage)
}
func AwsCredentialsChainVerboseErrors() bool {
	return awsCredentialsChainVerboseErrors
}
func AwsS3ForcePathStyle() bool {
	return awsS3ForcePathStyle
}
func DevMode() bool {
	return devMode
}
