package tests

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	awsClient "github.com/PayU/K8s-S3-Operator/controllers/aws"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-logr/logr"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var s3Client *s3.S3
var bucketName = "bucket-test-operator"
var namespace = "k8s-s3-operator-system"
var logger logr.Logger
var k8sClient client.Client
var s3Bucket s3operatorv1.S3Bucket
var timeToCreateBucket = time.Duration(5)

func TestMain(m *testing.M) {
	// run local env befor
	logger = zap.New(zap.UseFlagOptions(&zap.Options{})).
		WithName("system_test").
		WithValues("bucket_name", bucketName)

	os.Setenv("AWS_ENDPOINT", "http://localhost:80/")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	ses := awsClient.CreateSession(&logger)
	s3Client = awsClient.SetS3Client(&logger, ses)

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(s3operatorv1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Port:             9443,
		LeaderElectionID: "e8727534.payu.com",
	})
	if err != nil {
		logger.Error(err, "error create mew manager")
	} else {
		k8sClient = mgr.GetClient()
	}


	s3Bucket = s3operatorv1.S3Bucket{ObjectMeta: metav1.ObjectMeta{Name: bucketName, Namespace: namespace}}
	exitVal := m.Run()

	os.Exit(exitVal)
}

func TestCreateBucket(t *testing.T) {
	t.Log("TestCreateBucket")
	g := NewWithT(t)

	
	err := k8sClient.Create(context.Background(),&s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(timeToCreateBucket * time.Second)


}
func TestPutBucketData(t *testing.T) {
	t.Log("TestPutBucketData")
	g := NewWithT(t)

	_, err := s3Client.PutObject(&s3.PutObjectInput{Key: aws.String("testKey"),
		Body:   bytes.NewReader([]byte("test body")),
		Bucket: aws.String(bucketName)})
	g.Expect(err).NotTo(HaveOccurred())

}
func TestFetchBucketData(t *testing.T) {
	t.Log("TestFetchBucketData")
	g := NewWithT(t)

	res, err := s3Client.GetObject(&s3.GetObjectInput{Bucket: aws.String(bucketName), Key: aws.String("testKey")})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())

}
func TestDeleteBucketData(t *testing.T) {
	t.Log("TestDeleteBucketData")
	g := NewWithT(t)

	_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(bucketName), Key: aws.String("testKey")})
	g.Expect(err).NotTo(HaveOccurred())

}

func TestDeleteBucket(t *testing.T) {
	t.Log("TestDeleteBucket")
	g := NewWithT(t)

	err := k8sClient.Delete(context.Background(),&s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())

}
