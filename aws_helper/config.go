package aws_helper

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
		AssumeRoleTokenProvider: func() (string, error) {
			fmt.Print("Enter MFA Code: ")
			reader := bufio.NewReader(os.Stdin)
			mfa, err := reader.ReadString('\n')
			return strings.TrimSpace(mfa), err
		},
	}
	if awsRegion != "" {
		options.Config = aws.Config{Region: aws.String(awsRegion)}
	}
	session, err := session.NewSessionWithOptions(options)
	if err != nil {
		return nil, errors.WithStackTraceAndPrefix(err, "Error initializing session")
	}

	if os.Getenv("AWS_REGION") == "" && *session.Config.Region != "" {
		// If the default region is not set, we retain it
		os.Setenv("AWS_REGION", *session.Config.Region)
	}

	return session, nil
}

// InitAwsSession configures environment variables to ensure that all following AWS operations will be able to
// be executed using the proper credentials. Some calls to terraform library are not able to handle shared config
// properly. This also ensures that the session remains alive in case of MFA is required avoiding asking for
// MFA on each AWS calls.
func InitAwsSession(awsProfile string) (*session.Session, error) {
	if awsProfile != "" {
		// We unset the environment variables to not interfere with
		// the supplied profile
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("AWS_SESSION_TOKEN")
	}
	session, err := CreateAwsSession("", awsProfile)
	if err != nil {
		return nil, err
	}
	creds, err := session.Config.Credentials.Get()
	if err != nil {
		return nil, errors.WithStackTraceAndPrefix(err, "Error finding AWS credentials (did you set the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables?)")
	}
	os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)
	os.Unsetenv("AWS_PROFILE")
	os.Unsetenv("AWS_DEFAULT_PROFILE")
	return session, nil
}

// AssumeRoleEnvironmentVariables returns a set of key value pair to use as environment variables to assume a different role
func AssumeRoleEnvironmentVariables(roleArn string, sessionName string) (map[string]string, error) {
	if roleArn == "" {
		// If no role is specified, we just set AWS_SDK_LOAD_CONFIG to ensure that terraform will
		// use extended AWS Client configuration.
		os.Setenv("AWS_SDK_LOAD_CONFIG", "1")
		return nil, nil
	}
	session, err := CreateAwsSession("", "")
	if err != nil {
		return nil, err
	}

	if response, err := sts.New(session).AssumeRole(&sts.AssumeRoleInput{RoleArn: aws.String(roleArn), RoleSessionName: aws.String(sessionName)}); err == nil {
		return map[string]string{
			"AWS_ACCESS_KEY_ID":     *response.Credentials.AccessKeyId,
			"AWS_SECRET_ACCESS_KEY": *response.Credentials.SecretAccessKey,
			"AWS_SESSION_TOKEN":     *response.Credentials.SessionToken,
		}, nil
	}
	return nil, nil
}
