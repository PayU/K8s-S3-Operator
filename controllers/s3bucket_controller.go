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

	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	awsClient "github.com/PayU/K8s-S3-Operator/controllers/aws"

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
	var s3Bucket s3operatorv1.S3Bucket
	if err := r.Get(context.TODO(), req.NamespacedName, &s3Bucket); err != nil {
		bucketList := &s3operatorv1.S3BucketList{}
		err := r.List(context.TODO(), bucketList)
		if err != nil {
			r.Log.Error(err, "error on list s3 k8s resorces")
		} else {
			isDeleted, errInDeletion := r.AwsClient.HandleBucketDeletion(bucketList.Items)
			if !isDeleted && errInDeletion == nil { // error in getting s3 object
				r.Log.Error(err, "error with geting s3 bucket")
			} else {
				if isDeleted {
					r.Log.Info("succeded to delete bucket")
				} else {
					r.Log.Error(errInDeletion, "error in HandleBucketDeletion")
				}
			}

		}

		return ctrl.Result{Requeue: true}, nil
	}
	if !s3Bucket.Status.IsCreated {
		res, _ := r.AwsClient.HandleBucketCreation(&s3Bucket.Spec)
		s3Bucket.Status.IsCreated = res
		r.Status().Update(context.Background(), &s3Bucket)
	}

	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *S3BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&s3operatorv1.S3Bucket{}).
		Complete(r)
}
