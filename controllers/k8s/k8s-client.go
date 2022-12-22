package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"

	"github.com/PayU/K8s-S3-Operator/controllers/config"
	"github.com/go-logr/logr"
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
	k.Log.Info("starting to handle service account creation", "SA_name", serviceAcountName, "namespace", namespace, "iam_role", iamRole)
	//check if SA exsist
	sa, err := k.getServiceAccount(serviceAcountName, namespace)
	if err != nil {
		return err //unexpected error in get service account function
	}
	if sa == nil {
		sa, err = k.createServiceAccount(serviceAcountName, namespace, iamRole)
		if err == nil {
			k.Log.Info("succseded to create new service account")
			wait.ExponentialBackoff(wait.Backoff{Duration: config.WaitBackoffDuration(), Factor: config.WaitBackoffFactor(), Steps: config.WaitBackoffSteps()}, func() (done bool, err error) {
				err = k.checkMatchingAppToServiceAccount(serviceAcountName, s3Selector, namespace)
				return err == nil, nil
			})
			if err != nil {
				k.Log.Error(err, "error service account is not match to app")
				k.deleteServiceAccount(sa)
			}else{
				k.Add_SA_to_AC(serviceAcountName,namespace)
			}
		} else {
			k.Log.Error(err, "error to create new service account")
		}
		return err

	} else {
		err = k.checkMatchingAppToServiceAccount(serviceAcountName, s3Selector, namespace)
		if err != nil {
			k.Log.Error(err, "error service account is not match to app")
		} else {
			err = k.editServiceAccount(serviceAcountName, namespace, iamRole)
			k.Add_SA_to_AC(serviceAcountName,namespace)
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

func (k *K8sClient) checkMatchingAppToServiceAccount(SAName string, labelsFromS3 map[string]string, namespace string) error {
	appPods := v1.PodList{}
	//get all pods in namespace that match the labels
	err := k.List(context.Background(), &appPods, &client.ListOptions{Namespace: namespace, LabelSelector: labels.SelectorFromSet(labelsFromS3)})
	if err != nil {
		k.Log.Error(err, "error to list app pods", "labels", labelsFromS3)
		return err
	}
	if len(appPods.Items) == 0 {
		err = errors.New("no app match to labels")
		k.Log.Error(err, "no app match to labels", "labels", labelsFromS3)
		return err
	}
	for _, appPod := range appPods.Items {
		if appPod.Spec.ServiceAccountName != SAName {
			err = errors.New("app ServiceAccountName not match s3resource service account name")
			k.Log.Error(err, "app ServiceAccountName not match s3resource service account name", "appPod.Spec.ServiceAccountName", appPod.Spec.ServiceAccountName, "s3-resouce SA", SAName)
			return err
		}
	}
	k.Log.Info("service account is match to app", "serviceaccount name", SAName, "labels", labelsFromS3)

	return nil
}

func (k *K8sClient) deleteServiceAccount(sa *v1.ServiceAccount) error {

	err := k.Delete(context.Background(), sa)
	if err != nil {
		k.Log.Error(err, "error to delete service account", "serviceaccount name", sa.Name)
	}
	return err
}

func CheckIfNotFoundError(reqName string, errStr string) bool {
	pattern := reqName + "\" not found"
	match, _ := regexp.MatchString(pattern, errStr)
	return match

}
func (k *K8sClient) GetTokenFromSA(SAName string, namespace string) (string, error) {
	token, err := os.ReadFile(config.PathToToken())
	if err != nil {
		k.Log.Error(err,"error to read token","token_path",config.PathToToken())
		return "", err
	}
	k.Log.Info("succeded to get token","token",string(token))//todo remove log 
	return string(token), nil
}


func (k *K8sClient) Add_SA_to_AC(SAName string, namespace string)error{
	k.Log.Info("starting to add service account to AC")
	token, err := k.GetTokenFromSA(SAName, namespace)
	if err != nil{
		return err
	}
	httpClient := http.Client{}
	byteMap, err := json.Marshal(map[string]string{"img":"123"})
	if err != nil{
		k.Log.Error(err,"error to marshal")
		return err
	} 
	body := bytes.NewReader(byteMap)
	req, err := http.NewRequest("POST", config.PathToAC(),body)
	if err != nil {
		k.Log.Error(err,"error create request")
		return err
	}
	req.Header.Add("token",token)
	res, err := httpClient.Do(req)
	if err != nil {
		k.Log.Error(err,"error to post request")
		return err
	} 
	defer res.Body.Close()
	resBody ,_ :=ioutil.ReadAll(res.Body)
	k.Log.Info("succeded to add to AC", "res body", string(resBody))
	return nil

}
