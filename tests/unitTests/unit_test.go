package tests

import (
	"os"
	"testing"

	k8sClient "github.com/PayU/K8s-S3-Operator/controllers/k8s"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var logger logr.Logger

// unit test , test get value function

func TestMain(m *testing.M) {
	// run local env script befor test

	logger = zap.New(zap.UseFlagOptions(&zap.Options{})).
		WithName("system_test")
	exitVal := m.Run()
	logger.Info("finish to run all tests")

	os.Exit(exitVal)
}

func TestGetValueFunc(t *testing.T) {
	g := NewWithT(t)
	deploy := appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: v1.PodTemplateSpec{Spec: v1.PodSpec{ServiceAccountName: "NameOfServiceAccount"}}}}
	rightKey := "Spec.ServiceAccountName"
	wrongKey := "Spec.Spec.field"
	res,err := k8sClient.GetValue(rightKey,deploy,&logger)
	t.Log("err",err,"res",res,deploy)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).To(Equal(""))

	res,err = k8sClient.GetValue(wrongKey,deploy,&logger)
	g.Expect(err).To(HaveOccurred())
	g.Expect(res).To(Equal(nil))



}
