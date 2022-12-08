package controllers

import (
	"context"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClient struct {
	client.Client
	Log logr.Logger
}

// func (k *K8sClient)HandleSACreation()error{

// }

func (k *K8sClient) GetServiceAccount(serviceAcountName string, namespace string) (*v1.ServiceAccount,error) {
	sa := &v1.ServiceAccount{}
	err := k.Client.Get(context.Background(), types.NamespacedName{Name: serviceAcountName, Namespace: namespace}, sa)
	if err != nil {
		if CheckIfNotFoundError(serviceAcountName, err.Error()) {
			return nil, nil
		} else {
			k.Log.Error(err, "unexpcted error in Get in Reconcile function")
		}
	}
	return true
}

func (k *K8sClient) CreateServiceAccount(serviceAcountName string, namespace types.NamespacedName, iamRole string) bool {
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAcountName,
		Namespace:   namespace.Namespace,
		Annotations: map[string]string{"eks.amazonaws.com/role-arn": iamRole}}}

	err := k.Create(context.Background(), sa)
	if err != nil {
		return false
	}
	return true
}
func (k *K8sClient) EditServiceAccount(serviceAcountName string, namespace types.NamespacedName, iamRole string) bool {
	sa := &v1.ServiceAccount{}
	err := k.Get(context.Background(), namespace, sa)
	if err != nil {
		// sa.Annotations = sa.Annotations[]
		k.Update(context.Background(), sa)
	}
	return true
}
