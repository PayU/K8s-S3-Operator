package k8s

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
)

func validateResponseFromAuthServer(statusCode int, body string, Log *logr.Logger) (int, error) {
	if statusCode != 200 {
		err := errors.New("didnt succeded to add service account")
		Log.Error(err, "error from auth server", "statusCode", statusCode, "body", body)

		return statusCode, err
	}
	Log.Info("succeded to add to service account to  auth server", "statusCode", statusCode, "body", body)
	return statusCode, nil
}

func cerateMapForBody(dataMap map[string]string, appPod interface{}, Log *logr.Logger) map[string]string {
	for key, val := range dataMap {
		Log.V(1).Info("inside loop of setBody", "key", key, "val", val)
		val, err := getValue(val, appPod, Log)
		if err != nil {
			Log.Error(err, "error to get value")
			return nil
		}
		Log.V(1).Info("got value from get value func", "key", key, "val", val)
		dataMap[key] = fmt.Sprint(val)
	}
	return dataMap
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

func getValue(key string, obj interface{}, Log *logr.Logger) (ret interface{},err error) {
	var val reflect.Value
	// Use the recover function to handle panics.
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("got panic in function")
			Log.Error(err, "got panic in function", "r", r)
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
func CheckIfNotFoundError(reqName string, errStr string) bool {
	pattern := reqName + "\" not found"
	match, _ := regexp.MatchString(pattern, errStr)
	return match

}
