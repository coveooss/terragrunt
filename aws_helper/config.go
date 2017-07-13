package aws_helper

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gruntwork-io/terragrunt/errors"
)

// CreateAwsSession returns an AWS session object for the given region, ensuring that the credentials are available
func CreateAwsSession(awsRegion, awsProfile string) (*session.Session, error) {
	options := session.Options{
		Profile:           awsProfile,
		SharedConfigState: session.SharedConfigEnable,
	}
	if awsRegion != "" {
		options.Config = aws.Config{Region: aws.String(awsRegion)}
	}
	session, err := session.NewSessionWithOptions(options)

	if err != nil {
		return nil, errors.WithStackTraceAndPrefix(err, "Error initializing session")
	}

	_, err = session.Config.Credentials.Get()
	if err != nil {
		return nil, errors.WithStackTraceAndPrefix(err, "Error finding AWS credentials (did you set the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables?)")
	}

	return session, nil
}

// AssumeRoleEnvironmentVariables returns a set of key value pair to use as environment variables to assume a different role
func AssumeRoleEnvironmentVariables(roleArn string, sessionName string) (map[string]string, error) {
	session, err := CreateAwsSession("", "")
	if err != nil {
		return nil, err
	}

	svc := sts.New(session)
	response, err := svc.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
	})
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"AWS_ACCESS_KEY_ID":     *response.Credentials.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY": *response.Credentials.SecretAccessKey,
		"AWS_SESSION_TOKEN":     *response.Credentials.SessionToken,
	}, nil
}
