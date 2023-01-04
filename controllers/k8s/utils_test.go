package k8s

import (
	"os"
	"testing"


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
		WithName("unit_test").
		WithName("k8s_utils_test")
	exitVal := m.Run()
	logger.Info("finish to run all tests")

	os.Exit(exitVal)
}

func TestGetValueFuncRighrKey(t *testing.T) {
	g := NewWithT(t)
	deploy := appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: v1.PodTemplateSpec{Spec: v1.PodSpec{ServiceAccountName: "NameOfServiceAccount"}}}}
	rightKey := "Spec.template.spec.ServiceAccountName"
	res, err := getValue(rightKey, deploy, &logger)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).To(Equal("NameOfServiceAccount"))


}
func TestGetValueFuncWrongKey(t *testing.T) {
	g := NewWithT(t)
	deploy := appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: v1.PodTemplateSpec{Spec: v1.PodSpec{ServiceAccountName: "NameOfServiceAccount"}}}}
	wrongKey := "Spec.Spec.field"

	res, err := getValue(wrongKey, deploy, &logger)
	g.Expect(err).To(HaveOccurred())
	g.Expect(res).To(BeNil())

}

