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
	"time"

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	awsClient "github.com/PayU/K8s-S3-Operator/controllers/aws"
	k8s "github.com/PayU/K8s-S3-Operator/controllers/k8s"

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
	K8sClient *k8s.K8sClient
}

//+kubebuilder:rbac:groups=s3operator.payu.com,resources=s3buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts;pods;configmaps,verbs=get;list;watch;create;update;patch;delete

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
	log := r.Log.WithValues("namespace", req.Namespace, "bucket_name", req.Name)
	r.AwsClient.Log = &log
	var s3Bucket s3operatorv1.S3Bucket

	errToGet := r.Get(context.TODO(), req.NamespacedName, &s3Bucket)
	if errToGet != nil {
		var err error
		isDeleted := false
		if k8s.CheckIfNotFoundError(req.Name, errToGet.Error()) { // check if resource not exists
			isDeleted, err = r.handleDeleteFlow(&s3Bucket.Spec, req.Name, req.Namespace)
		} else { //unexpcted error
			log.Error(errToGet, "unexpcted error in Get in Reconcile function")
			err = errToGet
		}

		return ctrl.Result{Requeue: !isDeleted}, err
	}
	//succeded to get resource, check if need to create or update
	isbucketExists, err := r.AwsClient.IsBucketExists(s3Bucket.Name)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	if isbucketExists {
		err = r.handleUpdateFlow(&s3Bucket.Spec, s3Bucket.Name, req.Namespace)
	} else { //bucket not exists in aws, create
		err = r.handleCreationFlow(&s3Bucket.Spec, s3Bucket.Name, req.Namespace)
	}
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(10 * time.Second)}, err
	}
	return ctrl.Result{Requeue: false}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *S3BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&s3operatorv1.S3Bucket{}).
		Complete(r)
}

func (r *S3BucketReconciler) handleCreationFlow(bucketSpec *s3operatorv1.S3BucketSpec, bucketName string, namespace string) error {
	err := r.AwsClient.ValidateBucketName(bucketName)
	if err != nil {
		r.Log.Error(err, "bucket name is unvalid")
		return err
	}
	// create or update service account
	err = r.K8sClient.HandleSACreate(bucketSpec.Serviceaccount, namespace, awsClient.GetRoleName(bucketName), bucketSpec.Selector)
	if err != nil {
		return err
	}

	err = r.AwsClient.HandleBucketCreation(bucketSpec, bucketName, namespace)
	if err != nil {
		return err
	}
	return nil

}

func (r *S3BucketReconciler) handleUpdateFlow(bucketSpec *s3operatorv1.S3BucketSpec, bucketName string, namespace string) error {
	err := r.AwsClient.HandleBucketUpdate(bucketName, bucketSpec)
	return err
}

func (r *S3BucketReconciler) handleDeleteFlow(bucketSpec *s3operatorv1.S3BucketSpec, bucketName string, namespace string) (bool, error) {
	isDelted, err := r.AwsClient.HandleBucketDeletion(bucketName)
	return isDelted, err
}
