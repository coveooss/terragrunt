package aws_helper

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gruntwork-io/terragrunt/errors"
)

// CreateAwsSession returns an AWS session object for the given region, ensuring that the credentials are available
func CreateAwsSession(awsRegion, awsProfile string) (*session.Session, error) {
	mfaRequired := false
	options := session.Options{
		Profile:           awsProfile,
		SharedConfigState: session.SharedConfigEnable,
		AssumeRoleTokenProvider: func() (string, error) {
			mfaRequired = true
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

	creds, err := session.Config.Credentials.Get()
	if err != nil {
		return nil, errors.WithStackTraceAndPrefix(err, "Error finding AWS credentials (did you set the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables?)")
	}

	if mfaRequired {
		// If MFA has been required, we set the environment variables to keep the session alive and avoid
		// to be asked for MFA on every new session call during the current execution
		os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
		os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
		os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)
	}
	return session, nil
}

// AssumeRoleEnvironmentVariables returns a set of key value pair to use as environment variables to assume a different role
func AssumeRoleEnvironmentVariables(roleArn string, sessionName string) (result map[string]string, err error) {
	session, err := CreateAwsSession("", "")
	if err != nil {
		return nil, err
	}

	svc := sts.New(session)
	var creds credentials.Value

	if roleArn == "" {
		// If no role is specified, we just convert the current access to environment variables
		// if a role is assumed. This is required because terraform does not support AWS_PROFILE
		// that refers to a configuration that assume a role.
		creds, err = svc.Config.Credentials.Get()
	} else {
		var response *sts.AssumeRoleOutput
		response, err = svc.AssumeRole(&sts.AssumeRoleInput{
			RoleArn:         aws.String(roleArn),
			RoleSessionName: aws.String(sessionName),
		})
		if err != nil {
			return
		}
		creds = credentials.Value{
			AccessKeyID:     *response.Credentials.AccessKeyId,
			SecretAccessKey: *response.Credentials.SecretAccessKey,
			SessionToken:    *response.Credentials.SessionToken,
		}
	}

	result = map[string]string{
		"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
	}
	if creds.SessionToken != "" {
		result["AWS_SESSION_TOKEN"] = creds.SessionToken
	}
	return
}
