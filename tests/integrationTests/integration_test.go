package tests

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	utils "github.com/PayU/K8s-S3-Operator/tests/utils"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/gomega"
)

/*integration test will test 8 flows.
1. app not exsist, serviceaccount not exsist, s3bucket not exsist deploy app with Deployment
2. app already run with default serviceaccount, s3bucket not exsist => new deploy of bucket and update app serviceaccount
3. app already run with serviceaccount, s3bucket not exsist => deploy of bucket will update serviceaccount annotation
4. app not exsist, serviceaccount not exsist, s3bucket not exsist deploy app with statefullset
5. app not exsist, serviceaccount not exsist, s3bucket not exsist deploy app with Job
6. app not exsist, serviceaccount not exsist, s3bucket not exsist deploy app with DaemonSet
*/

var k8sClient client.Client
var logger logr.Logger
var pathKubectl string

const (
	appName            = "test-app"
	serviceAccountName = "s3-operator-test-app"
	s3BucketName       = "s3bucket-sample-app-testt"
	namespace          = "k8s-s3-operator-system"
	pathToAuthServer   = "http://localhost:4566/test-app"
	graceTime          = time.Duration(10)
	graceTimeAppChange = time.Duration(25)
)

func TestMain(m *testing.M) {
	logger = zap.New(zap.UseFlagOptions(&zap.Options{})).
		WithName("integration_test").
		WithValues("app_name", appName, "serviceAccount_name", serviceAccountName, "bucket_name", s3BucketName, "namespace", namespace)
	pathKubectl = FindKubectlPath()
	k8sClient = utils.CreateK8SClient(logger)

	exitVal := m.Run()
	logger.Info("finish to run all tests, return auth server to regular mode")
	K8sApply("./yamlFiles/deployAuthServer.yaml")
	os.Exit(exitVal)
}

func TestFlow1(t *testing.T) {
	t.Log("TestFlow1")

	g := NewWithT(t)
	t.Cleanup(Cleanup)
	//validate app deploy, serviceaccount, s3bucket not exsist
	validateResourceStatus(t, false, false, false, false, "deploy")

	// apply to k8s app deploy, serviceaccount, s3bucket
	err := K8sApply("./yamlFiles/testflow1.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	//check they created and running status
	validateResourceStatus(t, true, true, true, true, "deploy")

}

// TestFlow2: app already run with default serviceaccount, s3bucket not exsist => new deploy of bucket and update app serviceaccoun
func TestFlow2(t *testing.T) {
	t.Log("TestFlow2")
	t.Cleanup(Cleanup)
	g := NewWithT(t)
	//deploy app with defult service account  s3bucket not exsist
	err := K8sApply("./yamlFiles/tesflow2-start.yaml")
	g.Expect(err).NotTo(HaveOccurred())
	//validate begin status
	validateResourceStatus(t, true, false, false, true, "deploy")
	// apply to k8s app deploy, serviceaccount, s3bucket
	err = K8sApply("./yamlFiles/testflow2-update.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	validateResourceStatus(t, true, true, true, true, "deploy")
	//check they created and running status

}

// 3. app already run with serviceaccount, s3bucket not exsist => deploy of bucket will update serviceaccount annotation
func TestFlow3(t *testing.T) {
	t.Log("TestFlow3")
	t.Cleanup(Cleanup)
	g := NewWithT(t)
	//deploy app with  service account,  s3bucket not exsist
	err := K8sApply("./yamlFiles/testflow3-start.yaml")
	g.Expect(err).NotTo(HaveOccurred())
	//validate begin status
	validateResourceStatus(t, true, true, false, true, "deploy")
	// apply to k8s app deploy, serviceaccount, s3bucket
	err = K8sApply("./yamlFiles/testflow3-update.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	validateResourceStatus(t, true, true, true, true, "deploy")
	//check they created and running status

}

// 4. app not exsist, serviceaccount not exsist, s3bucket not exsist deploy app with statefullset
func TestFlow4(t *testing.T) {
	t.Log("TestFlow4")

	g := NewWithT(t)
	t.Cleanup(CleanupStatefulSet)
	//validate app deploy, serviceaccount, s3bucket not exsist
	validateResourceStatus(t, false, false, false, false, "statefulSet")

	// apply to k8s app deploy, serviceaccount, s3bucket
	err := K8sApply("./yamlFiles/testflow4.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	//check they created and running status
	validateResourceStatus(t, true, true, true, true, "statefulSet")

}

// 5. app not exsist, serviceaccount not exsist, s3bucket not exsist deploy app with Job
func TestFlow5(t *testing.T) {
	t.Log("TestFlow5")

	g := NewWithT(t)
	t.Cleanup(CleanupJob)
	//validate app deploy, serviceaccount, s3bucket not exsist
	validateResourceStatus(t, false, false, false, false, "job")

	// apply to k8s app deploy, serviceaccount, s3bucket
	err := K8sApply("./yamlFiles/testflow5.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	//check they created and running status
	validateResourceStatus(t, true, true, true, true, "job")

}

// 6. app not exsist, serviceaccount not exsist, s3bucket not exsist deploy app with DaemonSet
func TestFlow6(t *testing.T) {
	t.Log("TestFlow6")

	g := NewWithT(t)
	t.Cleanup(CleanupDemonSet)
	//validate app deploy, serviceaccount, s3bucket not exsist
	validateResourceStatus(t, false, false, false, false, "demonset")

	// apply to k8s app deploy, serviceaccount, s3bucket
	err := K8sApply("./yamlFiles/testflow6.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	//check they created and running status
	validateResourceStatus(t, true, true, true, true, "demonset")

}

// test with 500 and 403 from sa auth server
func TestRes500FromAuthServer(t *testing.T) {
	t.Log("TestRes500FromAuthServer")
	t.Cleanup(Cleanup)
	g := NewWithT(t)
	// setCounterToZero(t)
	//update auth server to err mode
	t.Log("update auth server to err mode")
	err := K8sApply("./yamlFiles/deployAuthServerErrMode.yaml")
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)
	//validate app deploy, serviceaccount, s3bucket not exsist
	validateResourceStatus(t, false, false, false, false, "deploy")
	// apply to k8s app deploy, serviceaccount, s3bucket
	err = K8sApply("./yamlFiles/testflow1.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	//check they created and running status
	validateResourceStatus(t, true, false, true, false, "deploy")

}

func TestRes403FromAuthServer(t *testing.T) {
	t.Log("TestRes403FromAuthServer")
	t.Cleanup(Cleanup)
	g := NewWithT(t)
	//update auth server to unauth mode
	t.Log("update auth server to unauth mode")
	err := K8sApply("./yamlFiles/deployAuthServerUnauthMode.yaml")
	g.Expect(err).NotTo(HaveOccurred())
	time.Sleep(graceTime * time.Second)
	//validate app deploy, serviceaccount, s3bucket not exsist
	validateResourceStatus(t, false, false, false, false, "deploy")
	// apply to k8s app deploy, serviceaccount, s3bucket
	err = K8sApply("./yamlFiles/testflow1.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	// time.Sleep(graceTimeAppChange * time.Second)

	//check they created and running status
	validateResourceStatus(t, true, false, true, false, "deploy")
	// validateNumOfCallToAuthServer(t, 1)

}
func validateResourceStatus(t *testing.T, expectPodController bool, expectSA bool, expectBucket bool, expectPods bool, podController string) {
	// g := NewWithT(t)
	switch podController {
	case "deploy":
		deploy := appsv1.Deployment{}
		getResourceEventually(t, &deploy, expectPodController, appName)
		if expectPodController {
			checkPods(t, expectPods)
		}
	case "statefulSet":
		sts := appsv1.StatefulSet{}
		getResourceEventually(t, &sts, expectPodController, appName)
		if expectPodController {
			checkPods(t, expectPods)
		}
	case "job":
		job := batchv1.Job{}
		getResourceEventually(t, &job, expectPodController, appName)

	case "demonset":
		demonset := appsv1.DaemonSet{}
		getResourceEventually(t, &demonset, expectPodController, appName)
		
	}

	sa := v1.ServiceAccount{}
	getResourceEventually(t, &sa, expectSA, serviceAccountName)
	s3Bucket := s3operatorv1.S3Bucket{}
	getResourceEventually(t, &s3Bucket, expectBucket, s3BucketName)

}

func Cleanup() {
	logger.Info("cleanup function")
	var err error

	deploy := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: namespace}}
	err = k8sClient.Delete(context.Background(), &deploy)
	HandleError(err, "error to delete deploy", "succeded to delete deploy")
	cleanupSAandBucket()

	logger.Info("finish cleanup")
}
func CleanupStatefulSet() {
	logger.Info("cleanup function")
	var err error
	sts := appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: namespace}}
	err = k8sClient.Delete(context.Background(), &sts)
	HandleError(err, "error to delete StatefulSet", "succeded to delete StatefulSet")
	cleanupSAandBucket()

	logger.Info("finish cleanup")
}
func CleanupJob() {
	logger.Info("cleanup function")
	var err error
	job := batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: namespace}}
	err = k8sClient.Delete(context.Background(), &job)
	HandleError(err, "error to delete job", "succeded to delete job")
	cleanupSAandBucket()

	logger.Info("finish cleanup")
}
func CleanupDemonSet() {
	logger.Info("cleanup function")
	var err error
	demonset := appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: namespace}}
	err = k8sClient.Delete(context.Background(), &demonset)
	HandleError(err, "error to delete demonset", "succeded to delete demonset")
	cleanupSAandBucket()
	logger.Info("finish cleanup")
}
func cleanupSAandBucket() {
	var err error
	sa := v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace}}
	s3Bucket := s3operatorv1.S3Bucket{ObjectMeta: metav1.ObjectMeta{Name: s3BucketName, Namespace: namespace}}
	err = k8sClient.Delete(context.Background(), &sa)
	HandleError(err, "error to delete serviceaccount", "succeded to delete serviceaccount")

	err = k8sClient.Delete(context.Background(), &s3Bucket)
	HandleError(err, "error to delete bucket", "succeded to delete bucket")
}
func HandleError(err error, msgError string, msgSucc string) {
	if err != nil {
		logger.Error(err, msgError)
	} else {
		logger.Info(msgSucc)
	}
}
func FindKubectlPath() string {
	path, err := exec.LookPath("kubectl")
	if err != nil {
		logger.Error(err, "error to find")
	} else {
		logger.Info(string(path))
	}
	return path

}
func K8sApply(pathToYaml string) error {
	_, err := exec.Command(pathKubectl, "apply", "-f", pathToYaml).Output()
	if err != nil {
		logger.Error(err, "error to apply yaml")
	} else {
		logger.Info("succeded to apply yaml file")
	}
	return err

}
func getResourceEventually(t *testing.T, k8sResource client.Object, expectToGet bool, name string) {
	g := NewWithT(t)
	t.Log("getResourceEventually", "expectToGet", expectToGet, name)
	g.Eventually(func() bool {
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, k8sResource)
		if expectToGet {
			return err == nil
		}
		return err != nil

	}, 20*time.Second, 4*time.Second).Should(Equal(true))
}
func checkPods(t *testing.T, expectPods bool) {
	g := NewWithT(t)
	deploy := appsv1.Deployment{}
	g.Eventually(func() bool {
		k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: appName}, &deploy)
		if expectPods {
			return deploy.Status.AvailableReplicas == *deploy.Spec.Replicas
		}
		return deploy.Status.AvailableReplicas == int32(0)
	}, 20*time.Second, 4*time.Second)

}
