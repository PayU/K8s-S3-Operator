/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"regexp"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	awsClient "github.com/PayU/K8s-S3-Operator/controllers/aws"
	"github.com/PayU/K8s-S3-Operator/controllers/k8sutils"


	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// S3BucketReconciler reconciles a S3Bucket object
type S3BucketReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Log       *logr.Logger
	AwsClient *awsClient.AwsClient
	k8sClient *k8sutils.K8sClient

}

//+kubebuilder:rbac:groups=s3operator.payu.com,resources=s3buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=s3operator.payu.com,resources=s3buckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=s3operator.payu.com,resources=s3buckets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the S3Bucket object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *S3BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.Namespace, "resource_name", req.Name)
	var s3Bucket s3operatorv1.S3Bucket
	// k := k8sutils.K8sClient{Client: r.Client,Log: &log}

	errToGet := r.Get(context.TODO(), req.NamespacedName, &s3Bucket)
	if errToGet != nil {
		var err error
		isDeleted := false
		if CheckIfNotFoundError(req.Name, errToGet.Error()) { // check if resource not exists
			isDeleted, err = r.AwsClient.HandleBucketDeletion(req.Name, &log)
		} else { //unexpcted error
			log.Error(errToGet, "unexpcted error in Get in Reconcile function")
			err = errToGet
		}

		return ctrl.Result{Requeue: !isDeleted}, err
	}
	//succeded to get resource, check if need to create or update
	isbucketExists, err := r.AwsClient.BucketExists(s3Bucket.Name, &log)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	if isbucketExists {
		_, err = r.AwsClient.HandleBucketUpdate(s3Bucket.Name, &s3Bucket.Spec, &log)
	} else { //bucket not exists in aws, create
		_, err = r.AwsClient.HandleBucketCreation(&s3Bucket.Spec, s3Bucket.Name, &log, req.Namespace)
	}
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{Requeue: false}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *S3BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&s3operatorv1.S3Bucket{}).
		Complete(r)
}

func CheckIfNotFoundError(reqName string, errStr string) bool {
	pattern := reqName + "\" not found"
	match, _ := regexp.MatchString(pattern, errStr)
	return match

}
