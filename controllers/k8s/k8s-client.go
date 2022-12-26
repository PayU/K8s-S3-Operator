package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

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
			} else {
				k.Add_SA_to_AC(serviceAcountName, namespace, s3Selector)
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
			k.Add_SA_to_AC(serviceAcountName, namespace, s3Selector)
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
	err := k.getPodList(namespace, labelsFromS3, &appPods)
	if err == nil {
		for _, appPod := range appPods.Items {
			if appPod.Spec.ServiceAccountName != SAName {
				err = errors.New("app ServiceAccountName not match s3resource service account name")
				k.Log.Error(err, "app ServiceAccountName not match s3resource service account name", "appPod.Spec.ServiceAccountName", appPod.Spec.ServiceAccountName, "s3-resouce SA", SAName)
				return err
			}
		}
		k.Log.Info("service account is match to app", "serviceaccount name", SAName, "labels", labelsFromS3)
	}

	return err
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
		k.Log.Error(err, "error to read token", "token_path", config.PathToToken())
		return "", err
	}
	k.Log.Info("succeded to get token", "token", string(token)) //todo remove log
	return string(token), nil
}

func (k *K8sClient) Add_SA_to_AC(SAName string, namespace string, labelsFromS3 map[string]string) error {
	k.Log.Info("starting to add service account to AC")
	token, err := k.GetTokenFromSA(SAName, namespace)
	if err != nil {
		return err
	}
	httpClient := http.Client{}

	req, err := http.NewRequest("POST", config.PathToAC(), k.setBody(namespace, labelsFromS3))
	if err != nil {
		k.Log.Error(err, "error create request")
		return err
	}
	req.Header.Add("token", token)
	res, err := httpClient.Do(req)
	if err != nil {
		k.Log.Error(err, "error to post request")
		return err
	}

	defer res.Body.Close()
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		k.Log.Error(err, "error to read body")
		return err
	}
	
	return validateResponse(res.StatusCode, string(resBody), k.Log)
}

func validateResponse(statusCode int, body string, Log *logr.Logger) error {
	if statusCode != 200 {
		err := errors.New("didnt succeded to add service account")
		Log.Error(err, "error from auth server", "statusCode", statusCode, "body", body)

		return err
	}
	Log.Info("succeded to add to service account to  auth server", "statusCode", statusCode, "body", body)
	return nil
}
func (k *K8sClient) setBody(namespace string, labelsFromS3 map[string]string) *bytes.Reader {
	// get config map that map the body of request
	cm, err := k.getConfigMap(config.ConfigMapName(), namespace)
	if err != nil {
		k.Log.Error(err, "error to get config map")
		return nil
	}
	appPods := v1.PodList{}
	err = k.getPodList(namespace, labelsFromS3, &appPods)
	if err != nil {
		return nil
	}
	dataMap := k.cerateMapForBody(cm.Data, appPods.Items[0])

	body := convertMapToByte(dataMap, k.Log)
	return body

}
func (k *K8sClient) getValue(key string, obj v1.Pod) (interface{}, error) {
	var val reflect.Value
	var err error
	// Use the recover function to handle panics.
	defer func() {
		k.Log.Info("dfer")
		if r := recover(); r != nil {
			err = errors.New("got panic in function")
			k.Log.Error(err, "got panic in function", "r", r)
		}
	}()

	splitedStr := strings.Split(key, ".")

	val = reflect.ValueOf(obj)
	for _, part := range splitedStr {
		part = toUpperFirstLetter(part)
		// Check if the value is a slice.
		if strings.Contains(part, "[") {
			// Extract the index from the part.
			index := strings.Index(part, "[")
			// Convert the index to an int.
			i, err := strconv.Atoi(part[index+1 : index+2])
			if err != nil {
				return nil, err
			}
			val = val.FieldByName(part[:index]).Index(i)

		} else {
			// Get the field with the specified name.
			val = val.FieldByName(part)

		}
	}
	return val.Interface(), err
}
func (k *K8sClient) cerateMapForBody(dataMap map[string]string, appPod v1.Pod) map[string]string {
	for key, val := range dataMap {
		k.Log.V(1).Info("inside loop of setBody", "key", key, "val", val)
		val, err := k.getValue(val, appPod)
		if err != nil {
			k.Log.Error(err, "error to get value")
			return nil
		}
		k.Log.V(1).Info("got value from get value func", "key", key, "val", val)
		dataMap[key] = fmt.Sprint(val)
	}
	return dataMap
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
func toUpperFirstLetter(str string) string {
	return strings.ToUpper(string(str[0])) + str[1:]
}
func convertMapToByte(dataMap map[string]string, Log *logr.Logger) *bytes.Reader {
	byteMap, err := json.Marshal(dataMap)
	if err != nil {
		Log.Error(err, "error to marshal")
		return nil
	}
	return bytes.NewReader(byteMap)

}
