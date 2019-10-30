package dynamodb

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/gruntwork-io/terragrunt/awshelper"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
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
func CreateDynamoDbClient(awsRegion, awsProfile string) (*dynamodb.DynamoDB, error) {
	session, err := awshelper.CreateAwsSession(awsRegion, awsProfile)
	if err != nil {
		return nil, err
	}

	return dynamodb.New(session), nil
}

// CreateLockTableIfNecessary creates the lock table in DynamoDB if it doesn't already exist
func CreateLockTableIfNecessary(tableName string, client *dynamodb.DynamoDB, terragruntOptions *options.TerragruntOptions) error {
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
func lockTableExistsAndIsActive(tableName string, client *dynamodb.DynamoDB) (bool, error) {
	output, err := client.DescribeTable(&dynamodb.DescribeTableInput{TableName: aws.String(tableName)})
	if err != nil {
		if awsErr, isAwsErr := err.(awserr.Error); isAwsErr && awsErr.Code() == "ResourceNotFoundException" {
			return false, nil
		}
		return false, errors.WithStackTrace(err)
	}

	return *output.Table.TableStatus == dynamodb.TableStatusActive, nil
}

// Create a lock table in DynamoDB and wait until it is in "active" state. If the table already exists, merely wait
// until it is in "active" state.
func createLockTable(tableName string, readCapacityUnits int, writeCapacityUnits int, client *dynamodb.DynamoDB, terragruntOptions *options.TerragruntOptions) error {
	tableCreateDeleteSemaphore.Acquire()
	defer tableCreateDeleteSemaphore.Release()

	terragruntOptions.Logger.Noticef("Creating table %s in DynamoDB", tableName)

	attributeDefinitions := []*dynamodb.AttributeDefinition{
		&dynamodb.AttributeDefinition{AttributeName: aws.String(attrLockID), AttributeType: aws.String(dynamodb.ScalarAttributeTypeS)},
	}

	keySchema := []*dynamodb.KeySchemaElement{
		&dynamodb.KeySchemaElement{AttributeName: aws.String(attrLockID), KeyType: aws.String(dynamodb.KeyTypeHash)},
	}

	_, err := client.CreateTable(&dynamodb.CreateTableInput{
		TableName:            aws.String(tableName),
		AttributeDefinitions: attributeDefinitions,
		KeySchema:            keySchema,
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(int64(readCapacityUnits)),
			WriteCapacityUnits: aws.Int64(int64(writeCapacityUnits)),
		},
	})

	if err != nil {
		if isTableAlreadyBeingCreatedError(err) {
			terragruntOptions.Logger.Warningf("Looks like someone created table %s at the same time. Will wait for it to be in active state.", tableName)
		} else {
			return errors.WithStackTrace(err)
		}
	}

	return waitForTableToBeActive(tableName, client, maxRetriesWaitingForTableToBeActive, sleepBetweenTableStatusChecks, terragruntOptions)
}

// DeleteTable deletes the given table in DynamoDB
func DeleteTable(tableName string, client *dynamodb.DynamoDB) error {
	tableCreateDeleteSemaphore.Acquire()
	defer tableCreateDeleteSemaphore.Release()

	_, err := client.DeleteTable(&dynamodb.DeleteTableInput{TableName: aws.String(tableName)})
	return err
}

// Return true if the given error is the error message returned by AWS when the resource already exists
func isTableAlreadyBeingCreatedError(err error) bool {
	awsErr, isAwsErr := err.(awserr.Error)
	return isAwsErr && awsErr.Code() == "ResourceInUseException"
}

// Wait for the given DynamoDB table to be in the "active" state. If it's not in "active" state, sleep for the
// specified amount of time, and try again, up to a maximum of maxRetries retries.
func waitForTableToBeActive(tableName string, client *dynamodb.DynamoDB, maxRetries int, sleepBetweenRetries time.Duration, terragruntOptions *options.TerragruntOptions) error {
	return waitForTableToBeActiveWithRandomSleep(tableName, client, maxRetries, sleepBetweenRetries, sleepBetweenRetries, terragruntOptions)
}

// Waits for the given table as described above, but sleeps a random amount of time greater than sleepBetweenRetriesMin
// and less than sleepBetweenRetriesMax between tries. This is to avoid an AWS issue where all waiting requests fire at
// the same time, which continually triggered AWS's "subscriber limit exceeded" API error.
func waitForTableToBeActiveWithRandomSleep(tableName string, client *dynamodb.DynamoDB, maxRetries int, sleepBetweenRetriesMin time.Duration, sleepBetweenRetriesMax time.Duration, terragruntOptions *options.TerragruntOptions) error {
	for i := 0; i < maxRetries; i++ {
		tableReady, err := lockTableExistsAndIsActive(tableName, client)
		if err != nil {
			return err
		}

		if tableReady {
			terragruntOptions.Logger.Noticef("Success! Table %s is now in active state.", tableName)
			return nil
		}

		sleepBetweenRetries := util.GetRandomTime(sleepBetweenRetriesMin, sleepBetweenRetriesMax)
		terragruntOptions.Logger.Warningf("Table %s is not yet in active state. Will check again after %s.", tableName, sleepBetweenRetries)
		time.Sleep(sleepBetweenRetries)
	}

	return errors.WithStackTrace(tableActiveRetriesExceededError{TableName: tableName, Retries: maxRetries})
}

type tableActiveRetriesExceededError struct {
	TableName string
	Retries   int
}

func (err tableActiveRetriesExceededError) Error() string {
	return fmt.Sprintf("Table %s is still not in active state after %d retries.", err.TableName, err.Retries)
}
