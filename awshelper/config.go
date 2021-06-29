package awshelper

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/coveooss/multilogger"
	"github.com/coveooss/terragrunt/v2/tgerrors"
)

// Environment variables used to publish the assumed role expiration date
const (
	EnvAssumedRole     = "TERRAGRUNT_ASSUMED_ROLE"
	EnvTokenExpiration = "TERRAGRUNT_TOKEN_EXPIRATION"
	EnvTokenDuration   = "TERRAGRUNT_TOKEN_DURATION"
)

var configCache sync.Map

func clearConfigCache() {
	configCache.Range(func(key interface{}, value interface{}) bool {
		configCache.Delete(key)
		return true
	})
}

// CreateAwsConfig returns an AWS config object for the given region, ensuring that the credentials are available
func CreateAwsConfig(awsRegion, awsProfile string) (*aws.Config, error) {
	loadOptions := []func(*config.LoadOptions) error{
		config.WithAssumeRoleCredentialOptions(func(options *stscreds.AssumeRoleOptions) {
			options.TokenProvider = stscreds.StdinTokenProvider
		}),
	}
	if awsRegion != "" {
		loadOptions = append(loadOptions, config.WithRegion(awsRegion))
	}
	if awsProfile != "" {
		loadOptions = append(loadOptions, config.WithSharedConfigProfile(awsProfile))
	}

	var (
		awsConfigKey = awsRegion + "/" + awsProfile
		awsConfig    aws.Config
		err          error
	)
	if cacheValue, ok := configCache.Load(awsConfigKey); ok {
		awsConfig = cacheValue.(aws.Config)
	} else {
		if awsConfig, err = config.LoadDefaultConfig(context.TODO(), loadOptions...); err != nil {
			return nil, tgerrors.WithStackTraceAndPrefix(err, "Error initializing AWS configuration")
		}
		configCache.Store(awsConfigKey, awsConfig)
	}

	if os.Getenv("AWS_DEFAULT_REGION") == "" && awsConfig.Region != "" {
		// If the default region is not set, we retain it
		os.Setenv("AWS_DEFAULT_REGION", awsConfig.Region)
	}

	return &awsConfig, nil
}

// InitAwsConfig configures environment variables to ensure that all following AWS operations will be able to
// be executed using the proper credentials. Some calls to terraform library are not able to handle shared config
// properly. This also ensures that the session remains alive in case of MFA is required avoiding asking for
// MFA on each AWS calls.
func InitAwsConfig(awsProfile string) (*aws.Config, error) {
	if awsProfile != "" {
		// We unset the environment variables to not interfere with
		// the supplied profile
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("AWS_SESSION_TOKEN")
	}
	config, err := CreateAwsConfig("", awsProfile)
	if err != nil {
		return nil, err
	}
	creds, err := config.Credentials.Retrieve(context.TODO())
	if err != nil {
		return nil, tgerrors.WithStackTraceAndPrefix(err, "Error finding AWS credentials (did you set the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables?)")
	}
	os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)
	os.Unsetenv("AWS_PROFILE")
	os.Unsetenv("AWS_DEFAULT_PROFILE")
	return config, nil
}

// AssumeRoleEnvironmentVariables returns a set of key value pair to use as environment variables to assume a different role
func AssumeRoleEnvironmentVariables(logger *multilogger.Logger, roleArn, sessionName string, assumeDuration *int) (map[string]string, error) {
	if roleArn == "" {
		// If no role is specified, we just set AWS_SDK_LOAD_CONFIG to ensure that terraform will
		// use extended AWS Client configuration.
		os.Setenv("AWS_SDK_LOAD_CONFIG", "1")
		return nil, nil
	}
	config, err := CreateAwsConfig("", "")
	if err != nil {
		return nil, err
	}

	if assumeDuration != nil {
		logger.Debugf("Trying to assume role `%s` with a %d hour duration", roleArn, *assumeDuration)
		if role, err := assumeRole(*config, roleArn, sessionName, int32(*assumeDuration*3600)); err != nil {
			logger.Debugf("Caught error assuming role `%s`: %s", roleArn, err)
		} else {
			return role, err
		}
	}

	logger.Debugf("Assuming role `%s` with a 1 hour duration", roleArn)
	return assumeRole(*config, roleArn, sessionName, 3600)
}

func assumeRole(config aws.Config, roleArn, sessionName string, durationSeconds int32) (map[string]string, error) {
	response, err := sts.NewFromConfig(config).AssumeRole(context.TODO(), &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
		DurationSeconds: aws.Int32(durationSeconds),
	})
	if err == nil {
		return map[string]string{
			"AWS_ACCESS_KEY_ID":     *response.Credentials.AccessKeyId,
			"AWS_SECRET_ACCESS_KEY": *response.Credentials.SecretAccessKey,
			"AWS_SESSION_TOKEN":     *response.Credentials.SessionToken,
			EnvAssumedRole:          *response.AssumedRoleUser.Arn,
			EnvTokenExpiration:      fmt.Sprint(response.Credentials.Expiration),
			EnvTokenDuration:        fmt.Sprint(time.Until(*response.Credentials.Expiration)),
		}, nil
	}
	return nil, err
}
