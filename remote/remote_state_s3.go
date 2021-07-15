package remote

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/coveooss/terragrunt/v2/awshelper"
	"github.com/coveooss/terragrunt/v2/dynamodb"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/mitchellh/mapstructure"
)

// StateConfigS3 is a representation of the configuration options available for S3 remote state
type StateConfigS3 struct {
	Encrypt   bool   `mapstructure:"encrypt"`
	Bucket    string `mapstructure:"bucket"`
	Key       string `mapstructure:"key"`
	Region    string `mapstructure:"region"`
	Profile   string `mapstructure:"profile"`
	LockTable string `mapstructure:"dynamodb_table"`
}

const maxRetriesWaitingForS3Bucket = 12
const sleepBetweenRetriesWaitingForS3Bucket = 5 * time.Second

// Initialize the remote state S3 bucket specified in the given config. This function will validate the config
// parameters, create the S3 bucket if it doesn't already exist, and check that versioning is enabled.
func initializeRemoteStateS3(config map[string]interface{}, terragruntOptions *options.TerragruntOptions) error {
	s3Config, err := parseS3Config(config)
	if err != nil {
		return err
	}

	if err := validateS3Config(s3Config, terragruntOptions); err != nil {
		return err
	}

	s3Client, err := CreateS3Client(s3Config.Region, s3Config.Profile)
	if err != nil {
		return err
	}

	if err := createS3BucketIfNecessary(s3Client, s3Config, terragruntOptions); err != nil {
		return err
	}

	if err := checkIfVersioningEnabled(s3Client, s3Config, terragruntOptions); err != nil {
		return err
	}

	if err := createLockTableIfNecessary(s3Config, terragruntOptions); err != nil {
		return err
	}

	return nil
}

// Parse the given map into an S3 config
func parseS3Config(config map[string]interface{}) (*StateConfigS3, error) {
	var s3Config StateConfigS3
	if err := mapstructure.Decode(config, &s3Config); err != nil {
		return nil, tgerrors.WithStackTrace(err)
	}

	return &s3Config, nil
}

// Validate all the parameters of the given S3 remote state configuration
func validateS3Config(config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	if config.Region == "" {
		return tgerrors.WithStackTrace(errMissingRequiredS3RemoteStateConfig("region"))
	}

	if config.Bucket == "" {
		return tgerrors.WithStackTrace(errMissingRequiredS3RemoteStateConfig("bucket"))
	}

	if config.Key == "" {
		return tgerrors.WithStackTrace(errMissingRequiredS3RemoteStateConfig("key"))
	}

	if !config.Encrypt {
		terragruntOptions.Logger.Warningf("Encryption is not enabled on the S3 remote state bucket %s. Terraform state files may contain secrets, so we STRONGLY recommend enabling encryption!", config.Bucket)
	}

	return nil
}

// If the bucket specified in the given config doesn't already exist, prompt the user to create it, and if the user
// confirms, create the bucket and enable versioning for it.
func createS3BucketIfNecessary(client *s3.Client, config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	if !DoesS3BucketExist(client, config) {
		prompt := fmt.Sprintf("Remote state S3 bucket %s does not exist or you don't have permissions to access it. Would you like Terragrunt to create it?", config.Bucket)
		shouldCreateBucket, err := shell.PromptUserForYesNo(prompt, terragruntOptions)
		if err != nil {
			return err
		}

		if shouldCreateBucket {
			return createS3BucketWithVersioning(client, config, terragruntOptions)
		}
	}

	return nil
}

// Check if versioning is enabled for the S3 bucket specified in the given config and warn the user if it is not
func checkIfVersioningEnabled(client *s3.Client, config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	out, err := client.GetBucketVersioning(context.TODO(), &s3.GetBucketVersioningInput{Bucket: aws.String(config.Bucket)})
	if err != nil {
		return tgerrors.WithStackTrace(err)
	}

	if out.Status != types.BucketVersioningStatusEnabled {
		terragruntOptions.Logger.Warningf("Versioning is not enabled for the remote state S3 bucket %s. We recommend enabling versioning so that you can roll back to previous versions of your Terraform state in case of error.", config.Bucket)
	}

	return nil
}

// Create the given S3 bucket and enable versioning for it
func createS3BucketWithVersioning(client *s3.Client, config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	if err := createS3Bucket(client, config, terragruntOptions); err != nil {
		return err
	}

	if err := waitUntilS3BucketExists(client, config, terragruntOptions); err != nil {
		return err
	}

	if err := enableVersioningForS3Bucket(client, config, terragruntOptions); err != nil {
		return err
	}

	return nil
}

// AWS is eventually consistent, so after creating an S3 bucket, this method can be used to wait until the information
// about that S3 bucket has propagated everywhere
func waitUntilS3BucketExists(client *s3.Client, config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	for retries := 0; retries < maxRetriesWaitingForS3Bucket; retries++ {
		if DoesS3BucketExist(client, config) {
			terragruntOptions.Logger.Infof("S3 bucket %s created.", config.Bucket)
			return nil
		} else if retries < maxRetriesWaitingForS3Bucket-1 {
			terragruntOptions.Logger.Warningf("S3 bucket %s has not been created yet. Sleeping for %s and will check again.", config.Bucket, sleepBetweenRetriesWaitingForS3Bucket)
			time.Sleep(sleepBetweenRetriesWaitingForS3Bucket)
		}
	}

	return tgerrors.WithStackTrace(errMaxRetriesWaitingForS3BucketExceeded(config.Bucket))
}

// Create the S3 bucket specified in the given config
func createS3Bucket(client *s3.Client, config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	terragruntOptions.Logger.Infof("Creating S3 bucket %s in %s", config.Bucket, config.Region)
	_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(config.Bucket),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(config.Region),
		},
	})
	return tgerrors.WithStackTrace(err)
}

// Enable versioning for the S3 bucket specified in the given config
func enableVersioningForS3Bucket(client *s3.Client, config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	terragruntOptions.Logger.Infoln("Enabling versioning on S3 bucket", config.Bucket)
	input := s3.PutBucketVersioningInput{
		Bucket:                  aws.String(config.Bucket),
		VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatusEnabled},
	}
	_, err := client.PutBucketVersioning(context.TODO(), &input)
	return tgerrors.WithStackTrace(err)
}

// DoesS3BucketExist returns true if the S3 bucket specified in the given config exists and the current user has the ability to access it.
func DoesS3BucketExist(client *s3.Client, config *StateConfigS3) bool {
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{Bucket: aws.String(config.Bucket)})
	return err == nil
}

// Create a table for locks in DynamoDB if the user has configured a lock table and the table doesn't already exist
func createLockTableIfNecessary(s3Config *StateConfigS3, terragruntOptions *options.TerragruntOptions) error {
	if s3Config.LockTable == "" {
		return nil
	}

	dynamodbClient, err := dynamodb.CreateDynamoDbClient(s3Config.Region, s3Config.Profile)
	if err != nil {
		return err
	}

	return dynamodb.CreateLockTableIfNecessary(s3Config.LockTable, dynamodbClient, terragruntOptions)
}

// CreateS3Client creates an authenticated client for S3.
func CreateS3Client(awsRegion, awsProfile string) (*s3.Client, error) {
	config, err := awshelper.CreateAwsConfig(awsRegion, awsProfile)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(*config), nil
}

// Custom error types
type errMissingRequiredS3RemoteStateConfig string

func (configName errMissingRequiredS3RemoteStateConfig) Error() string {
	return fmt.Sprintf("Missing required S3 remote state configuration %s", string(configName))
}

type errMaxRetriesWaitingForS3BucketExceeded string

func (err errMaxRetriesWaitingForS3BucketExceeded) Error() string {
	return fmt.Sprintf("Exceeded max retries (%d) waiting for bucket S3 bucket %s", maxRetriesWaitingForS3Bucket, string(err))
}
