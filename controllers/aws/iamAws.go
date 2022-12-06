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
	IamClient *iam.IAM
	Log       *logr.Logger
}

func (c IamClient) CreateIamRole(roleName string, Tag *iam.Tag, log *logr.Logger) (*iam.CreateRoleOutput, error) {
	c.Log = log
	c.Log.Info("Creating IAM role for s3 bucket", "role_name", roleName)
	policy, err := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Action": []string{
					"s3:*",
				},
				"Effect":   "Allow",
				"Resource": "*",
			},
		},
	})
	if err != nil {
		c.Log.Error(err, "error in CreateIamRole in Marshal")
		return nil, err
	}
	input := iam.CreateRoleInput{
		RoleName:                 &roleName,
		Tags:                     []*iam.Tag{Tag},
		AssumeRolePolicyDocument: aws.String(string(policy)),
	}
	res, err := c.IamClient.CreateRole(&input)
	if err != nil {
		c.Log.Error(err, "error in CreateIamRole in CreateRole", "role_name", roleName)
	} else {
		c.Log.Info("Create IAM for s3 bucket finished successfully", "role_from_res", res.Role)
	}
	return res, err
}

func (c IamClient) DeleteIamRole(roleName string, log *logr.Logger) (*iam.DeleteRoleOutput, error) {
	c.Log = log
	c.Log.Info("DeleteIamRole function", "role_name", roleName)
	input := iam.DeleteRoleInput{
		RoleName: &roleName,
	}
	res, err := c.IamClient.DeleteRole(&input)
	if err != nil {
		c.Log.Error(err, "error in DeleteIamRole in DeleteRole", "role_name", roleName)
	} else {
		c.Log.Info("succeded to delete iam role", "role_name", roleName)
	}
	return res, err
}
func setIamClient(Log *logr.Logger, ses *session.Session) *iam.IAM {
	Log.Info("create iamClient wit session", "session", *ses)
	iamClient := iam.New(ses)
	if iamClient == nil {
		iamClienttErr := errors.New("error in create iamClient")
		Log.Error(iamClienttErr, "didnt succeded to create iamClient")
	} else {
		Log.Info(" succeded create iamClient", "client", *iamClient)
	}
	return iamClient

}
