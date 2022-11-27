package aws

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/go-logr/logr"
)

type IamClient struct {
	IamClient iam.IAM
	Log       logr.Logger
}

func (c IamClient) CreateIamRole(roleName string,Tag *iam.Tag) (*iam.CreateRoleOutput, error) {
	c.Log.Info("CreateIamRole function")
	policy,err := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
		  {
			"Action":[]string{
			  "s3:*",
			},
			"Effect":"Allow",
			"Resource":"*",
		  },
		},
	  })
	if err != nil{
		c.Log.Error(err, "error in CreateIamRole in Marshal")
		return nil, err
	}
	input := iam.CreateRoleInput{
		RoleName: &roleName,
		Tags: []*iam.Tag{Tag},
		AssumeRolePolicyDocument: aws.String(string(policy)),
	}
	res, err := c.IamClient.CreateRole(&input)
	if err != nil {
		c.Log.Error(err, "error in CreateIamRole in CreateRole")
	}else{
		c.Log.Info("succeded to create iam role", "res", res)
	}
	return res, err
}

func (c IamClient) DeleteIamRole(roleName string) (*iam.DeleteRoleOutput, error) {
	c.Log.Info("DeleteIamRole function")
	input := iam.DeleteRoleInput{
		RoleName: &roleName,
	}
	res, err := c.IamClient.DeleteRole(&input)
	if err != nil {
		c.Log.Error(err, "error in DeleteIamRole in DeleteRole")
	}else{
		c.Log.Info("succeded to delete iam role", "res", res)
	}
	return res, err
}
func setIamClient(Log *logr.Logger, ses *session.Session) iam.IAM {
	iamClient := iam.New(ses)
	if iamClient == nil {
		iamClienttErr := errors.New("error in create RGTAClient")
		Log.Error(iamClienttErr, "didnt succeded to create s3Client")
	} else {
		Log.Info(" succeded create RGTAClient", "client", *iamClient)
	}
	return *iamClient

}
