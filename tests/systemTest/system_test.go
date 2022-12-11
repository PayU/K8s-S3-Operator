package tests

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
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
var graceTime = time.Duration(5)

func TestMain(m *testing.M) {
	// run local env script befor test

	logger = zap.New(zap.UseFlagOptions(&zap.Options{})).
		WithName("system_test").
		WithValues("bucket_name", bucketName)

	logger.Info("start to run before test configuration")

	ses := awsClient.CreateSession(&logger)
	s3Client = awsClient.SetS3Client(&logger, ses)
	s3Client.Endpoint = "http://localhost:4566"

	createK8SClient()

	s3Bucket = s3operatorv1.S3Bucket{ObjectMeta: metav1.ObjectMeta{Name: bucketName, Namespace: namespace}}
	exitVal := m.Run()
	logger.Info("finish to run all tests")

	os.Exit(exitVal)
}

func createK8SClient() {
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
}

func TestCreateBucket(t *testing.T) {
	t.Log("start TestCreateBucket")
	g := NewWithT(t)
	t.Log("check bucket not exsits")
	_, err := s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(bucketName)})
	g.Expect(err).To(HaveOccurred())

	t.Log("create new bucket resource", "bucket_name", bucketName)

	err = k8sClient.Create(context.Background(), &s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)

	t.Log("check bucket exsits")
	_, err = s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(bucketName)})
	g.Expect(err).NotTo(HaveOccurred())

	t.Log("finish TestCreateBucket")

}
func TestBucketUpdateTag(t *testing.T) {
	t.Log("start TestBucketUpdateTag")
	g := NewWithT(t)

	t.Log("get bucket tagging expect not to have error and the only tag is the default tag ")
	tags, err := s3Client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: aws.String(bucketName)})
	g.Expect(err).NotTo(HaveOccurred())
	t.Log("got tags from aws", tags.TagSet)
	g.Expect(len(tags.TagSet)).Should(Equal(1))

	t.Log("update bucket tags expect not to have error and to add the new tag")
	s3Bucket.Spec.Tags = map[string]string{"testKey": "testValue"}
	err = k8sClient.Update(context.Background(), &s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)
	tags, err = s3Client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: aws.String(bucketName)})
	t.Log("got tags from aws", tags.TagSet)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(len(tags.TagSet)).Should(Equal(2))

	t.Log("finish TestBucketUpdateTag")

}

func TestPutBucketData(t *testing.T) {
	t.Log("start TestPutBucketData")
	g := NewWithT(t)

	t.Log("put data object in bucket expect not to have error")
	_, err := s3Client.PutObject(&s3.PutObjectInput{Key: aws.String("testKey"),
		Body:   bytes.NewReader([]byte("test body")),
		Bucket: aws.String(bucketName)})
	g.Expect(err).NotTo(HaveOccurred())

}
func TestFetchBucketData(t *testing.T) {
	t.Log("TestFetchBucketData")
	g := NewWithT(t)

	t.Log("get data object with key:testKey expect not to have error")
	res, err := s3Client.GetObject(&s3.GetObjectInput{Bucket: aws.String(bucketName), Key: aws.String("testKey")})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())
	//check that getting body of object
	buf := new(strings.Builder)
	io.Copy(buf, res.Body)
	g.Expect(buf.String()).To(Equal("test body"))

	t.Log("finish TestPutBucketData")
}
func TestDeleteBucketData(t *testing.T) {
	t.Log("start TestDeleteBucketData")
	g := NewWithT(t)

	t.Log("delete data object with key:testKey expect not to have error")
	_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(bucketName), Key: aws.String("testKey")})
	g.Expect(err).NotTo(HaveOccurred())

	t.Log("finish TestDeleteBucketData")

}

func TestDeleteBucket(t *testing.T) {
	t.Log("start TestDeleteBucket")
	g := NewWithT(t)

	t.Log("check bucket exsist expect not to have error", bucketName)
	_, err := s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(bucketName)})
	g.Expect(err).NotTo(HaveOccurred())

	t.Log("delete bucket resource expect not to have error")
	err = k8sClient.Delete(context.Background(), &s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)

	t.Log("check bucket exsist expect to get error", bucketName)
	_, err = s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(bucketName)})
	g.Expect(err).To(HaveOccurred())

	t.Log("finish TestDeleteBucket")

}
