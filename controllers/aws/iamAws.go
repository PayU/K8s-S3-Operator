package aws

import (
	"github.com/aws/aws-sdk-go/service/iam"
)

type IamClient struct {
	client iam.IAM
}

func (c IamClient) CreateIamRole(roleName string) (*iam.CreateRoleOutput, error) {
	input := iam.CreateRoleInput{
		RoleName: &roleName,
	}
	res, err := c.client.CreateRole(&input)

	return res, err
}

func (c IamClient) DeleteIamRole(roleName string) (*iam.DeleteRoleOutput, error) {
	input := iam.DeleteRoleInput{
		RoleName: &roleName,
	}
	res, err := c.client.DeleteRole(&input)

	return res, err
}
