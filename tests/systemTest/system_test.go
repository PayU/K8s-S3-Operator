package tests

import (
	"os"
	"testing"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	awsClient "github.com/PayU/K8s-S3-Operator/controllers/aws"




)

func TestMain(m *testing.M){
	// run local env befor 
	ses := awsClient.CreateSession()
	s3Client := awsClient.setS3Client(ses)
	
	
	exitVal := m.Run()


	os.Exit(exitVal)
}

func TestCreateBucket(t *testing.T) {
	t.Log("TestCreateBucket")
}
func TestPutBucketData(t *testing.T) {
	t.Log("TestPutBucketData")

}
func TestFetchBucketData(t *testing.T) {
	t.Log("TestFetchBucketData")
}
func TestDeleteBucket(t *testing.T) {
	t.Log("TestDeleteBucket")
}
