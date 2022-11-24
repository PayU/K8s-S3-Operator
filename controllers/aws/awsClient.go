package aws

import (
	"errors"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"

	"github.com/aws/aws-sdk-go/service/s3"
)

type AwsClient struct {
	s3Client s3.S3
	Log      logr.Logger
	rMap     ResourceMap
}

var AWS_END_POINT = "http://172.19.0.4:31566"

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

func (a *AwsClient) HandleBucketCreation(bucketSpec *s3operatorv1.S3BucketSpec,resourceName string) (bool, error) {
	a.Log.Info("HandleBucketCreation")

	BucketExist, err := a.BucketExists(bucketSpec.BucketName)
	if BucketExist {
		a.Log.Error(err, "bucket allredy exsist")
		return false, errors.New("bucket allredy exsist")
	}
	bucketInput := a.CreateBucketInput(bucketSpec.BucketName, bucketSpec.Region)
	res, err := a.CreateBucket(*bucketInput)
	if err != nil {
		a.Log.Error(err, "got error in create bucket function")
		return false, err
	}
	// todo: insert bucket policy
	if bucketSpec.Encryption {
		sucEncrypt, err := a.PutBucketEncrypt(bucketSpec.BucketName)
		// todo: think about deleteing bucket when not succsed to encrypt
		if err != nil {
			a.Log.Error(err, "error to encrypt")
			return true, err
		} else {
			if !sucEncrypt {
				return true, errors.New("didnt succseded to encrypt")
			}
		}
	}
	a.rMap.AddResource(resourceName,bucketSpec.BucketName)
	a.Log.Info("succeded to create new bucket", "createBucketOutput", res)
	a.getAllBucket()
	return true, nil

}

func (a *AwsClient) HandleBucketDeletion(resourceName string) (bool, error) {
	bucketName := a.rMap.GetBucketNameForResource(resourceName)
	a.Log.Info("bucket name in HandleBucketDeletion is "+bucketName)
	res, err := a.DeleteBucket(bucketName)
	if err != nil {
		a.Log.Error(err, "err delete bucket")
		return false, err
	}
	a.Log.Info("succeded to delete bucket", "res", res)
	a.rMap.RemoveResource(resourceName)
	return true, nil
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

func (a *AwsClient) CreateBucketInput(bucketName string, bucketRegion string) *s3.CreateBucketInput {
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	s3Input.CreateBucketConfiguration = &s3.CreateBucketConfiguration{LocationConstraint: aws.String(bucketRegion)}
	a.Log.Info("bucketInput", "bucketInput", s3Input)
	return s3Input
}

func (a *AwsClient) DeleteBucket(bucketName string) (*s3.DeleteBucketOutput, error) {
	res, err := a.s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	return res, err
}

func (a *AwsClient) PutBucketPolicy(bucketName string, roleName string) (*s3.PutBucketPolicyOutput, error) {

	bucketPolicy := `{'Version':${} ,'Statement': [{ 'Sid': 'id-1','Effect': 'Allow','Principal': {'AWS': 'arn:aws:iam::123456789012:root'}, 'Action': [ 's3:PutObject','s3:PutObjectAcl'], 'Resource': ['arn:aws:s3:::acl3/*' ] } ]}`

	input := &s3.PutBucketPolicyInput{
		Bucket: &bucketName,
		Policy: &bucketPolicy,
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

	input := &s3.DeleteBucketPolicyInput{
		Bucket: &bucketName,
	}
	res, err := a.s3Client.DeleteBucketPolicy(input)
	return res, err
}

func (a *AwsClient) getAllBucket() {
	buckets, err := a.s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		a.Log.Error(err, "not succeded to get buckets")
	} else {
		a.Log.Info("ALL BUCKETS", "buckets", buckets)
	}
}

func CreateSession() *session.Session {
	ses := session.Must(session.NewSession(&aws.Config{
		Region:                        aws.String(endpoints.UsEast1RegionID),
		S3ForcePathStyle:              aws.Bool(true),
		Endpoint:                      aws.String(AWS_END_POINT),
		CredentialsChainVerboseErrors: aws.Bool(true),
		DisableSSL:                    aws.Bool(true),
	},
	))
	return ses
}

func setS3Client(Log *logr.Logger) s3.S3 {
	ses := CreateSession()
	if ses == nil {
		err := errors.New("ses is nil")
		Log.Error(err, "error in create new session")
	}
	Log.Info("session", "ses", *ses)
	s3Client := s3.New(ses)
	if s3Client == nil {
		s3ClientErr := errors.New("Error in create s3 client")
		Log.Error(s3ClientErr, "didnt succeded to create s3Client")

	} else {
		Log.Info(" succeded create s3Client", "client", *s3Client)
	}
	return *s3Client
}

func GetAwsClient(logger *logr.Logger) *AwsClient {
	return &AwsClient{
		s3Client: setS3Client(logger),
		Log:      *logger,
		rMap: ResourceMap{},
	}

}
