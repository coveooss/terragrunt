package dynamodb

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/stretchr/testify/assert"
)

var mockOptions = options.NewTerragruntOptionsForTest("dynamo_lock_test_utils")

// Returns a unique (ish) id we can use to name resources so they don't conflict with each other. Uses base 62 to
// generate a 6 character string that's unlikely to collide with the handful of tests we run in parallel. Based on code
// here: http://stackoverflow.com/a/9543797/483528
func uniqueID() string {
	const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const uniqueIDLength = 6 // Should be good for 62^6 = 56+ billion combinations

	var out bytes.Buffer

	for i := 0; i < uniqueIDLength; i++ {
		out.WriteByte(base62Chars[rand.Intn(len(base62Chars))])
	}

	return out.String()
}

// Create a DynamoDB client we can use at test time. If there are any errors creating the client, fail the test.
func createDynamoDbClientForTest(t *testing.T) *dynamodb.Client {
	// We always use us-east-1 for test purpose
	client, err := CreateDynamoDbClient("us-east-1", "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func uniqueTableNameForTest() string {
	return fmt.Sprintf("terragrunt_test_%s", uniqueID())
}

func cleanupTableForTest(t *testing.T, tableName string, client *dynamodb.Client) {
	err := DeleteTable(tableName, client)
	assert.Nil(t, err, "Unexpected error: %v", err)
}

func assertCanWriteToTable(t *testing.T, tableName string, client *dynamodb.Client) {
	item := createKeyFromItemID(uniqueID())

	_, err := client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	assert.Nil(t, err, "Unexpected error: %v", err)
}

func withLockTable(t *testing.T, action func(tableName string, client *dynamodb.Client)) {
	client := createDynamoDbClientForTest(t)
	tableName := uniqueTableNameForTest()

	err := CreateLockTableIfNecessary(tableName, client, mockOptions)
	assert.Nil(t, err, "Unexpected error: %v", err)
	defer cleanupTableForTest(t, tableName, client)

	action(tableName, client)
}

func createKeyFromItemID(itemID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		attrLockID: &types.AttributeValueMemberS{Value: itemID},
	}
}
