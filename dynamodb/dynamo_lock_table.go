package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/coveooss/terragrunt/v2/awshelper"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
)

// DynamoDB only allows 10 table creates/deletes simultaneously. To ensure we don't hit this error, especially when
// running many automated tests in parallel, we use a counting semaphore
var tableCreateDeleteSemaphore = newCountingSemaphore(10)

// Terraform requires the DynamoDB table to have a primary key with this name
const attrLockID = "LockID"

// Default is to retry for up to 5 minutes
const maxRetriesWaitingForTableToBeActive = 30
const sleepBetweenTableStatusChecks = 10 * time.Second

const defaultReadCapacityUnits = 1
const defaultWriteCapacityUnits = 1

// CreateDynamoDbClient creates an authenticated client for DynamoDB
func CreateDynamoDbClient(awsRegion, awsProfile string) (*dynamodb.Client, error) {
	config, err := awshelper.CreateAwsConfig(awsRegion, awsProfile)
	if err != nil {
		return nil, err
	}

	return dynamodb.NewFromConfig(*config), nil
}

// CreateLockTableIfNecessary creates the lock table in DynamoDB if it doesn't already exist
func CreateLockTableIfNecessary(tableName string, client *dynamodb.Client, terragruntOptions *options.TerragruntOptions) error {
	tableExists, err := lockTableExistsAndIsActive(tableName, client)
	if err != nil {
		return err
	}

	if !tableExists {
		terragruntOptions.Logger.Warningf("Lock table %s does not exist in DynamoDB. Will need to create it just this first time.", tableName)
		return createLockTable(tableName, defaultReadCapacityUnits, defaultWriteCapacityUnits, client, terragruntOptions)
	}

	return nil
}

// Return true if the lock table exists in DynamoDB and is in "active" state
func lockTableExistsAndIsActive(tableName string, client *dynamodb.Client) (bool, error) {
	output, err := client.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{TableName: aws.String(tableName)})
	if err != nil {
		if errors.Is(err, &types.ResourceNotFoundException{}) {
			return false, nil
		}
		return false, tgerrors.WithStackTrace(err)
	}

	return output.Table.TableStatus == types.TableStatusActive, nil
}

// Create a lock table in DynamoDB and wait until it is in "active" state. If the table already exists, merely wait
// until it is in "active" state.
func createLockTable(tableName string, readCapacityUnits int, writeCapacityUnits int, client *dynamodb.Client, terragruntOptions *options.TerragruntOptions) error {
	tableCreateDeleteSemaphore.Acquire()
	defer tableCreateDeleteSemaphore.Release()

	terragruntOptions.Logger.Infof("Creating table %s in DynamoDB", tableName)

	attributeDefinitions := []types.AttributeDefinition{
		{AttributeName: aws.String(attrLockID), AttributeType: types.ScalarAttributeTypeS},
	}

	keySchema := []types.KeySchemaElement{
		{AttributeName: aws.String(attrLockID), KeyType: types.KeyTypeHash},
	}

	_, err := client.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName:            aws.String(tableName),
		AttributeDefinitions: attributeDefinitions,
		KeySchema:            keySchema,
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(int64(readCapacityUnits)),
			WriteCapacityUnits: aws.Int64(int64(writeCapacityUnits)),
		},
	})

	if err != nil {
		if errors.Is(err, &types.ResourceInUseException{}) {
			terragruntOptions.Logger.Warningf("Looks like someone created table %s at the same time. Will wait for it to be in active state.", tableName)
		} else {
			return tgerrors.WithStackTrace(err)
		}
	}

	return waitForTableToBeActive(tableName, client, maxRetriesWaitingForTableToBeActive, sleepBetweenTableStatusChecks, terragruntOptions)
}

// DeleteTable deletes the given table in DynamoDB
func DeleteTable(tableName string, client *dynamodb.Client) error {
	tableCreateDeleteSemaphore.Acquire()
	defer tableCreateDeleteSemaphore.Release()

	_, err := client.DeleteTable(context.TODO(), &dynamodb.DeleteTableInput{TableName: aws.String(tableName)})
	return err
}

// Wait for the given DynamoDB table to be in the "active" state. If it's not in "active" state, sleep for the
// specified amount of time, and try again, up to a maximum of maxRetries retries.
func waitForTableToBeActive(tableName string, client *dynamodb.Client, maxRetries int, sleepBetweenRetries time.Duration, terragruntOptions *options.TerragruntOptions) error {
	return waitForTableToBeActiveWithRandomSleep(tableName, client, maxRetries, sleepBetweenRetries, sleepBetweenRetries, terragruntOptions)
}

// Waits for the given table as described above, but sleeps a random amount of time greater than sleepBetweenRetriesMin
// and less than sleepBetweenRetriesMax between tries. This is to avoid an AWS issue where all waiting requests fire at
// the same time, which continually triggered AWS's "subscriber limit exceeded" API error.
func waitForTableToBeActiveWithRandomSleep(tableName string, client *dynamodb.Client, maxRetries int, sleepBetweenRetriesMin time.Duration, sleepBetweenRetriesMax time.Duration, terragruntOptions *options.TerragruntOptions) error {
	for i := 0; i < maxRetries; i++ {
		tableReady, err := lockTableExistsAndIsActive(tableName, client)
		if err != nil {
			return err
		}

		if tableReady {
			terragruntOptions.Logger.Infof("Success! Table %s is now in active state.", tableName)
			return nil
		}

		sleepBetweenRetries := util.GetRandomTime(sleepBetweenRetriesMin, sleepBetweenRetriesMax)
		terragruntOptions.Logger.Warningf("Table %s is not yet in active state. Will check again after %s.", tableName, sleepBetweenRetries)
		time.Sleep(sleepBetweenRetries)
	}

	return tgerrors.WithStackTrace(tableActiveRetriesExceededError{TableName: tableName, Retries: maxRetries})
}

type tableActiveRetriesExceededError struct {
	TableName string
	Retries   int
}

func (err tableActiveRetriesExceededError) Error() string {
	return fmt.Sprintf("Table %s is still not in active state after %d retries.", err.TableName, err.Retries)
}
