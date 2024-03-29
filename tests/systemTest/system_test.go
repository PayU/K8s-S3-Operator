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
	utils "github.com/PayU/K8s-S3-Operator/tests/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-logr/logr"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var s3Client *s3.S3
var bucketName = "bucket-test-operator"
var notManageBucketName = "not-manage-bucket-test-operator"
var namespace = "k8s-s3-operator-system"
var logger logr.Logger
var k8sClient client.Client
var s3Bucket s3operatorv1.S3Bucket
var s3BucketNotManage s3operatorv1.S3Bucket
var graceTime = time.Duration(7)
var serviceAccountName = "system-test-serviceaccount"
var appName = "system-test-app"

func TestMain(m *testing.M) {
	// run local env script befor test

	logger = zap.New(zap.UseFlagOptions(&zap.Options{})).
		WithName("system_test").
		WithValues("bucket_name", bucketName)

	logger.Info("start to run before test configuration")

	ses := awsClient.CreateSession(&logger)
	s3Client = awsClient.SetS3Client(&logger, ses)
	s3Client.Endpoint = "http://localhost:4566/localstack"

	k8sClient = utils.CreateK8SClient(logger)

	s3Bucket = s3operatorv1.S3Bucket{ObjectMeta: metav1.ObjectMeta{Name: bucketName, Namespace: namespace},
		Spec: s3operatorv1.S3BucketSpec{Serviceaccount: serviceAccountName, Selector: map[string]string{"app": appName}}}
	s3BucketNotManage = s3operatorv1.S3Bucket{ObjectMeta: metav1.ObjectMeta{Name: notManageBucketName, Namespace: namespace},
		Spec: s3operatorv1.S3BucketSpec{Serviceaccount: serviceAccountName, Selector: map[string]string{"app": appName}}}
	var exitVal int
	for i := 1; i <= 3; i++ { // retry to pass tests
		logger.Info("Run system tests", "tryNumber", i)
		exitVal = m.Run()
		if exitVal == 0 {
			logger.Info("pass all test", "tryNumber", i)
			break
		} else {
			cleanup()
			time.Sleep(graceTime * time.Second)
		}
	}
	logger.Info("finish to run all tests")
	cleanup()
	os.Exit(exitVal)
}

func TestCreateBucket(t *testing.T) {
	t.Log("start TestCreateBucket")
	checkBucketExsist(t,bucketName,false)

	t.Log("create new bucket resource", "bucket_name", bucketName)

	createBucketResource(t,&s3Bucket)

	checkBucketExsist(t,bucketName,true)

	t.Log("finish TestCreateBucket")

}
func TestBucketUpdateTag(t *testing.T) {
	t.Log("start TestBucketUpdateTag")

	checkBucketTags(t,bucketName,1)
	updateBucketTags(t,bucketName,&s3Bucket)
	checkBucketTags(t,bucketName,2)

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
	deleteBucket(t,bucketName,&s3Bucket)

	checkBucketExsist(t, bucketName, false)

	t.Log("finish TestDeleteBucket")

}
func TestTryCreateBucketExsistNotManage(t *testing.T) {
	t.Log("start TestTryCreateBucketExsistNotManage")
	g := NewWithT(t)

	_,err := s3Client.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String(notManageBucketName)})
	g.Expect(err).NotTo(HaveOccurred())
	checkBucketExsist(t, notManageBucketName, true)

	createBucketResource(t,&s3BucketNotManage)
	checkBucketStatus(t,notManageBucketName,&s3BucketNotManage,"failed")

}
func TestValidateUpdateOnlyManageBuckets(t *testing.T) {
	t.Log("start TestValidateUpdateOnlyManageBuckets")
	checkBucketExsist(t, notManageBucketName, true)
	updateBucketTags(t, notManageBucketName,&s3BucketNotManage)
	checkBucketStatus(t,notManageBucketName,&s3BucketNotManage,"failed")

}

func TestValidateDeleteOnlyManageBuckets(t *testing.T) {
	t.Log("start TestValidateDeleteOnlyManageBuckets")
	deleteBucket(t,notManageBucketName,&s3BucketNotManage)

	checkBucketExsist(t, notManageBucketName, true)

	t.Log("finish TestValidateDeleteOnlyManageBuckets")

}

func checkBucketExsist(t *testing.T, bucketName string, expectBucket bool) {
	g := NewWithT(t)
	_, err := s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(bucketName)})
	if expectBucket {
		t.Log("check bucket exsist expect not to have error", bucketName)
		g.Expect(err).NotTo(HaveOccurred())
	} else {
		t.Log("check bucket exsist expect to get error", bucketName)
		g.Expect(err).To(HaveOccurred())

	}
}
func createBucketResource(t *testing.T,s3Bucket *s3operatorv1.S3Bucket){
	g := NewWithT(t)
	err := k8sClient.Create(context.Background(), s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)
}
func updateBucketTags(t *testing.T, bucketName string, s3Bucket *s3operatorv1.S3Bucket) {
	g := NewWithT(t)
	t.Log("update bucket tags expect not to have error and to add the new tag")
	err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: bucketName}, s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())

	s3Bucket.Spec.Tags = map[string]string{"testKey": "testValue"}
	err = k8sClient.Update(context.TODO(), s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)
}

func deleteBucket(t *testing.T, bucketName string,s3Bucket *s3operatorv1.S3Bucket){
	g := NewWithT(t)
	checkBucketExsist(t, bucketName, true)

	t.Log("delete bucket resource expect not to have error")
	err := k8sClient.Delete(context.Background(), s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)
}
func checkBucketTags(t *testing.T,bucketName string, expectedTags int){
	g := NewWithT(t)
	t.Log("get bucket tagging expect not to have error ")
	tags, err := s3Client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: aws.String(bucketName)})
	g.Expect(err).NotTo(HaveOccurred())
	t.Log("got tags from aws", tags.TagSet)
	g.Expect(len(tags.TagSet)).Should(Equal(expectedTags))
}
func checkBucketStatus(t *testing.T, bucketName string,s3Bucket *s3operatorv1.S3Bucket, expectedSatus string){
	g := NewWithT(t)
	err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: bucketName}, s3Bucket)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s3Bucket.Status.Status).Should(Equal(expectedSatus))

}
func cleanup(){
	s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(notManageBucketName)})

	k8sClient.Delete(context.Background(),&s3Bucket)
	k8sClient.Delete(context.Background(),&s3BucketNotManage)


}