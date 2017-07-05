package util

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// GetSSMParameter returns the value from the parameters store
func GetSSMParameter(parameterName string) (string, error) {
	withDecryption := true
	svc := ssm.New(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})))
	result, err := svc.GetParameter(&ssm.GetParameterInput{
		Name:           &parameterName,
		WithDecryption: &withDecryption,
	})

	if err != nil {
		return "", err
	}

	return *result.Parameter.Value, err
}
