package k8sutils

import (
	"context"
	"errors"
	"regexp"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClient struct {
	client.Client
	Log *logr.Logger
}

func (k *K8sClient) HandleSACreate(serviceAcountName string, namespace string, iamRole string,s3Selector map[string]string) error {
	k.Log.Info("starting handle create of service account", "SA_name", serviceAcountName, "namespace", namespace, "iam_role", iamRole)
	//check if SA exsist
	err := k.ValidateServiceAccount(serviceAcountName,s3Selector,namespace)
	if err != nil{
		return err
	}
	sa, err := k.GetServiceAccount(serviceAcountName, namespace)
	if err != nil {
		return err //unexpected error in get service account function
	}
	if sa == nil {
		err = k.CreateServiceAccount(serviceAcountName, namespace, iamRole)
	} else {

		err = k.EditServiceAccount(serviceAcountName, namespace, iamRole)
	}
	return err

}

func (k *K8sClient) GetServiceAccount(serviceAcountName string, namespace string) (*v1.ServiceAccount, error) {
	sa := &v1.ServiceAccount{}
	err := k.Client.Get(context.Background(), types.NamespacedName{Name: serviceAcountName, Namespace: namespace}, sa)
	if err != nil {
		if CheckIfNotFoundError(serviceAcountName, err.Error()) {
			return nil, nil
		} else {
			k.Log.Error(err, "unexpcted error in Get in Reconcile function")
			return nil, err
		}
	}
	return sa, nil
}

func (k *K8sClient) CreateServiceAccount(serviceAcountName string, namespace string, iamRole string) error {
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAcountName,
		Namespace:   namespace,
		Annotations: map[string]string{"eks.amazonaws.com/role-arn": iamRole}}}

	err := k.Create(context.Background(), sa)
	if err != nil {
		k.Log.Error(err, "error in create service account resource")
		return err
	}
	return nil
}

func (k *K8sClient) EditServiceAccount(serviceAcountName string, namespace string, iamRole string) error {
	sa := &v1.ServiceAccount{}
	err := k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: serviceAcountName}, sa)
	if err != nil {
		k.Log.Error(err, "error in get service account resource")
		return err
	}
	if val, found := sa.Annotations["eks.amazonaws.com/role-arn"]; found {
		if val == iamRole {
			k.Log.Info("service account allready have this iam role", "iam_role", iamRole)
			return nil
		}
		err = errors.New("iam role annotation allready exsist, need to update role")
		return err
	}

	sa.Annotations["eks.amazonaws.com/role-arn"] = iamRole
	err = k.Update(context.Background(), sa)
	if err != nil {
		k.Log.Error(err, "error in update service account resource")
		return err
	}
	return nil
}

func (k *K8sClient) ValidateServiceAccount(SAName string, labelsFromS3 map[string]string, namespace string) error {
	appPods := v1.PodList{}
	//get all pods in namespace that match the labels
	err := k.List(context.Background(), &appPods, &client.ListOptions{Namespace: namespace, LabelSelector: labels.SelectorFromSet(labelsFromS3)})
	if err != nil {
		k.Log.Error(err, "error to list app pods", "labels", labelsFromS3)
		return err
	}
	for _, appPod := range appPods.Items {
		if appPod.Spec.ServiceAccountName != SAName {
			err = errors.New("app ServiceAccountName not match s3resource service account name")
			k.Log.Error(err, "app ServiceAccountName not match s3resource service account name", "appPod.Spec.ServiceAccountName", appPod.Spec.ServiceAccountName, "s3-resouce SA", SAName)
			return err
		}
	}

	return nil
}

func CheckIfNotFoundError(reqName string, errStr string) bool {
	pattern := reqName + "\" not found"
	match, _ := regexp.MatchString(pattern, errStr)
	return match

}
