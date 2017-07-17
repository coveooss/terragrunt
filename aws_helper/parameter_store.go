package aws_helper

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// GetSSMParameter returns the value from the parameters store
func GetSSMParameter(parameterName, region string) (string, error) {
	session, err := CreateAwsSession(region, "")
	if err != nil {
		return "", err
	}

	svc := ssm.New(session)
	withDecryption := true
	result, err := svc.GetParameter(&ssm.GetParameterInput{
		Name:           &parameterName,
		WithDecryption: &withDecryption,
	})

	if err != nil {
		return "", err
	}

	return *result.Parameter.Value, err
}

// GetSSMParametersByPath returns values from the parameters store matching the path
func GetSSMParametersByPath(path, region string) (result []*ssm.Parameter, err error) {
	session, err := CreateAwsSession(region, "")
	if err != nil {
		return
	}

	svc := ssm.New(session)

	response, err := svc.GetParametersByPath(&ssm.GetParametersByPathInput{
		Path:           aws.String(path),
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(true),
	})

	if err != nil {
		return
	}

	result = response.Parameters
	return
}
