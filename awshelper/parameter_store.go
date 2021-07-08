package awshelper

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// GetSSMParameter returns the value from the parameters store
func GetSSMParameter(parameterName, region string) (string, error) {
	config, err := CreateAwsConfig(region, "")
	if err != nil {
		return "", err
	}

	svc := ssm.NewFromConfig(*config)
	result, err := svc.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &parameterName,
		WithDecryption: true,
	})

	if err != nil {
		return "", err
	}

	return *result.Parameter.Value, err
}

// GetSSMParametersByPath returns values from the parameters store matching the path
func GetSSMParametersByPath(path, region string) (result []types.Parameter, err error) {
	config, err := CreateAwsConfig(region, "")
	if err != nil {
		return
	}

	svc := ssm.NewFromConfig(*config)

	response, err := svc.GetParametersByPath(context.TODO(), &ssm.GetParametersByPathInput{
		Path:           aws.String(path),
		Recursive:      true,
		WithDecryption: true,
	})

	if err != nil {
		return
	}

	result = response.Parameters
	return
}
