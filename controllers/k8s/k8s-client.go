package k8s

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/PayU/K8s-S3-Operator/controllers/config"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClient struct {
	client.Client
	Log *logr.Logger
}

func (k *K8sClient) HandleSACreate(serviceAcountName string, namespace string, iamRole string, s3Selector map[string]string) error {
	k.Log.Info("starting to handle service account creation", "serviceAcount Name", serviceAcountName, "namespace", namespace, "iam_role", iamRole)
	//check if SA - service account exsist
	sa, err := k.getServiceAccount(serviceAcountName, namespace)
	if err != nil {
		return err //unexpected error in get service account function
	}
	if sa == nil { //service account not exists
		sa, err = k.createServiceAccount(serviceAcountName, namespace, iamRole)
		if err == nil {
			k.Log.Info("succseded to create new service account")
			err = wait.ExponentialBackoff(wait.Backoff{Duration: config.WaitBackoffDuration(), Factor: config.WaitBackoffFactor(), Steps: config.WaitBackoffSteps()}, func() (done bool, err error) {
				err = k.checkMatchingAppControllerToServiceAccount(serviceAcountName, s3Selector, namespace)
				k.Log.Info("in ExponentialBackoff checkMatchingAppToServiceAccount", "WaitBackoffDuration", config.WaitBackoffDuration(), "factor", config.WaitBackoffFactor(), "steps", config.WaitBackoffSteps(), "err", err)
				return err == nil, err
			})
			// after service account created
			if err != nil {
				k.Log.Error(err, "error service account is not match to app")
				k.deleteServiceAccount(sa)
			} else { // adding to service account to auth server
				var statuscode int
				err = wait.ExponentialBackoff(wait.Backoff{Duration: config.WaitBackoffDuration(), Factor: config.WaitBackoffFactor(), Steps: config.WaitBackoffSteps()}, func() (done bool, err error) {
					statuscode, err = k.addSAToAuthServer(serviceAcountName, namespace, s3Selector)
					k.Log.Info("in ExponentialBackoff", "statuscode", statuscode, "err", err)
					if statuscode == 403 {
						return true, err
					}
					return err == nil, err
				})
				if err != nil { // didnt succeded to add service account to auth server
					k.Log.Error(err, "error to add service account to auth server")
					k.deleteServiceAccount(sa)
				}

			}
		} else {
			k.Log.Error(err, "error to create new service account")
		}
		return err

	} else { //service accoun exsist
		err = k.checkMatchingAppControllerToServiceAccount(serviceAcountName, s3Selector, namespace)
		if err != nil {
			k.Log.Error(err, "error service account is not match to app")
		} else {
			err = k.editServiceAccount(serviceAcountName, namespace, iamRole)
		}

	}
	return err
}

func (k *K8sClient) getServiceAccount(serviceAcountName string, namespace string) (*v1.ServiceAccount, error) {
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

func (k *K8sClient) createServiceAccount(serviceAcountName string, namespace string, iamRole string) (*v1.ServiceAccount, error) {
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAcountName,
		Namespace:   namespace,
		Annotations: map[string]string{"eks.amazonaws.com/role-arn": iamRole}}}

	err := k.Create(context.Background(), sa)
	if err != nil {
		k.Log.Error(err, "error in create service account resource")
		return nil, err
	}
	return sa, nil
}

func (k *K8sClient) editServiceAccount(serviceAcountName string, namespace string, iamRole string) error {
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

func (k *K8sClient) checkMatchingAppControllerToServiceAccount(SAName string, labelsFromS3 map[string]string, namespace string) error {
	noMatchError := errors.New("app ServiceAccountName not match s3resource service account name")

	//try to find deploy
	deploy := appsv1.Deployment{}
	err := k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &deploy)
	if err == nil {
		if deploy.Spec.Template.Spec.ServiceAccountName == SAName {
			return nil
		}
		return noMatchError
	}
	//try to find statefull set
	sts := appsv1.StatefulSet{}
	err = k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &sts)
	if err == nil {
		if sts.Spec.Template.Spec.ServiceAccountName == SAName {
			return nil
		}
		return noMatchError
	}
	//try to find job
	job := batchv1.Job{}
	err = k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &job)
	if err == nil {
		if job.Spec.Template.Spec.ServiceAccountName == SAName {
			return nil
		}
		return noMatchError
	}
	//try to find demonset
	demonset := appsv1.DaemonSet{}
	err = k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &demonset)
	if err == nil {
		if demonset.Spec.Template.Spec.ServiceAccountName == SAName {
			return nil
		}
		return noMatchError
	}

	err = errors.New("didnt find any match pod controller")
	k.Log.Error(err, "serviceaccount name", SAName, "labels", labelsFromS3)
	return err
}

func (k *K8sClient) deleteServiceAccount(sa *v1.ServiceAccount) error {
	k.Log.Info("Delete service account", "serviceaccount name", sa.Name)
	err := k.Delete(context.Background(), sa)
	if err != nil {
		k.Log.Error(err, "error to delete service account", "serviceaccount name", sa.Name)
	}
	return err
}

func (k *K8sClient) getTokenFromSA(SAName string, namespace string) (string, error) {
	token, err := os.ReadFile(config.PathToToken())
	if err != nil {
		k.Log.Error(err, "error to read token", "token_path", config.PathToToken())
		return "", err
	}
	k.Log.Info("succeded to get token")
	return string(token), nil
}

func (k *K8sClient) addSAToAuthServer(SAName string, namespace string, labelsFromS3 map[string]string) (int, error) {
	k.Log.Info("starting to add service account to AC")
	token, err := k.getTokenFromSA(SAName, namespace)
	if err != nil {
		return 0, err
	}
	httpClient := http.Client{}

	req, err := http.NewRequest("POST", config.ServiceAccountApprovalUrl(), k.setBody(namespace, labelsFromS3))
	if err != nil {
		k.Log.Error(err, "error create request")
		return 0, err
	}
	req.Header.Add("token", token)
	res, err := httpClient.Do(req)
	if err != nil {
		k.Log.Error(err, "error to post request")
		return 0, err
	}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		k.Log.Error(err, "error to read body")
		return 0, err
	}
	return validateResponseFromAuthServer(res.StatusCode, string(resBody), k.Log)

}

func (k *K8sClient) setBody(namespace string, labelsFromS3 map[string]string) *bytes.Reader {
	// get config map that map the body of request
	cm, err := k.getConfigMap(config.ConfigMapName(), namespace)
	if err != nil {
		k.Log.Error(err, "error to get config map")
		return nil
	}

	podController := k.findPodsController(namespace, labelsFromS3)
	if podController == nil {
		k.Log.Error(errors.New("didnt find pod controller"), "error to gfind pod controller", "podController name", labelsFromS3["app"])
		return nil
	}
	dataMap := cerateMapForBody(cm.Data, podController, k.Log)

	body := convertMapToByte(dataMap, k.Log)
	return body

}
func (k *K8sClient) findPodsController(namespace string, labelsFromS3 map[string]string) interface{} {
	//try to find deploy
	deploy := appsv1.Deployment{}
	err := k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &deploy)
	if err == nil {
		return deploy
	}
	//try to find statefull set
	sts := appsv1.StatefulSet{}
	err = k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &sts)
	if err == nil {
		return sts
	}

	//try to find job
	job := batchv1.Job{}
	err = k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &job)
	if err == nil {
		return job
	}

	//try to find demonset
	demonset := appsv1.DaemonSet{}
	err = k.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: labelsFromS3["app"]}, &demonset)
	if err == nil {
		return demonset
	}
	return nil
}

func (k *K8sClient) getConfigMap(configMapName string, namespace string) (*v1.ConfigMap, error) {
	cm := &v1.ConfigMap{}
	err := k.Client.Get(context.Background(), types.NamespacedName{Name: configMapName, Namespace: namespace}, cm)
	if err != nil {
		k.Log.Error(err, "error to get config map")
		return nil, err
	}

	return cm, nil
}
func (k *K8sClient) getPodList(namespace string, labelsFromS3 map[string]string, appPods *v1.PodList) error {
	//get all pods in namespace that match the labels
	err := k.List(context.Background(), appPods, &client.ListOptions{Namespace: namespace, LabelSelector: labels.SelectorFromSet(labelsFromS3)})
	if err != nil {
		k.Log.Error(err, "error to list app pods", "labels", labelsFromS3)
		return err
	}
	if len(appPods.Items) == 0 {
		err = errors.New("no app match to labels")
		k.Log.Error(err, "no app match to labels", "labels", labelsFromS3)
		return err
	}
	return nil
}
