package test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
)

// hard-code this to match the test fixture for now
const (
	TEST_FIXTURE_HOOKS_BEFORE_ONLY_PATH            = "fixture-hooks/before-only"
	TEST_FIXTURE_HOOKS_AFTER_ONLY_PATH             = "fixture-hooks/after-only"
	TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_PATH       = "fixture-hooks/before-and-after"
	TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_MERGE_PATH = "fixture-hooks/before-and-after-merge"
	TEST_FIXTURE_HOOKS_INTERPOLATIONS_PATH         = "fixture-hooks/interpolations"
	TEST_FIXTURE_HOOKS_EXITCODE1_PATH              = "fixture-hooks/exitcode-1"
	TEST_FIXTURE_HOOKS_EXITCODE2_PATH              = "fixture-hooks/exitcode-2"
)

func TestTerragruntBeforeHook(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, TEST_FIXTURE_HOOKS_BEFORE_ONLY_PATH)
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_HOOKS_BEFORE_ONLY_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_HOOKS_BEFORE_ONLY_PATH)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))

	_, exception := ioutil.ReadFile(rootPath + "/file.out")

	assert.NoError(t, exception)
}

func TestTerragruntAfterHook(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, TEST_FIXTURE_HOOKS_AFTER_ONLY_PATH)
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_HOOKS_AFTER_ONLY_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_HOOKS_AFTER_ONLY_PATH)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))

	_, exception := ioutil.ReadFile(rootPath + "/file.out")

	assert.NoError(t, exception)
}

func TestTerragruntBeforeAndAfterHook(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_PATH)
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_PATH)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))

	_, beforeException := ioutil.ReadFile(rootPath + "/before.out")
	_, afterException := ioutil.ReadFile(rootPath + "/after.out")

	assert.NoError(t, beforeException)
	assert.NoError(t, afterException)
}

func TestTerragruntBeforeAndAfterMergeHook(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_MERGE_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_MERGE_PATH)
	childPath := util.JoinPath(rootPath, TEST_FIXTURE_INCLUDE_CHILD_REL_PATH)
	cleanupTerraformFolder(t, childPath)

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueId()))
	t.Logf("bucketName: %s", s3BucketName)
	defer deleteS3Bucket(t, TERRAFORM_REMOTE_STATE_S3_REGION, s3BucketName)

	tmpTerragruntConfigPath := createTmpTerragruntConfigWithParentAndChild(t, TEST_FIXTURE_HOOKS_BEFORE_AND_AFTER_MERGE_PATH, TEST_FIXTURE_INCLUDE_CHILD_REL_PATH, s3BucketName, config.DefaultTerragruntConfigPath, config.DefaultTerragruntConfigPath)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-config %s --terragrunt-working-dir %s", tmpTerragruntConfigPath, childPath))

	_, beforeException := ioutil.ReadFile(childPath + "/before.out")
	_, beforeChildException := ioutil.ReadFile(childPath + "/before-child.out")
	_, beforeOverriddenParentException := ioutil.ReadFile(childPath + "/before-parent.out")
	_, afterException := ioutil.ReadFile(childPath + "/after.out")
	_, afterParentException := ioutil.ReadFile(childPath + "/after-parent.out")

	assert.NoError(t, beforeException)
	assert.NoError(t, beforeChildException)
	assert.NoError(t, afterException)
	assert.NoError(t, afterParentException)

	// PathError because no file found
	assert.Error(t, beforeOverriddenParentException)
}

func TestTerragruntHookInterpolation(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, TEST_FIXTURE_HOOKS_INTERPOLATIONS_PATH)
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_HOOKS_INTERPOLATIONS_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_HOOKS_INTERPOLATIONS_PATH)

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	err := runTerragruntCommand(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	homePath := os.Getenv("HOME")
	if homePath == "" {
		homePath = "HelloWorld"
	}

	if err != nil {
		t.Errorf("Did not expect to get error: %s", err.Error())
	}

	assert.Contains(t, output, homePath)

}

func TestTerragruntHookExitCode1(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, TEST_FIXTURE_HOOKS_EXITCODE1_PATH)
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_HOOKS_EXITCODE1_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_HOOKS_EXITCODE1_PATH)

	err := runTerragruntCommand(t, fmt.Sprintf("terragrunt plan -detailed-exitcode --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), os.Stdout, os.Stderr)

	_, exception := ioutil.ReadFile(rootPath + "/test.out")
	assert.Error(t, exception)
	assert.Contains(t, err.Error(), "Error while executing hooks(post_hook_1): exit status 1")
}

func TestTerragruntHookExitCode2(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, TEST_FIXTURE_HOOKS_EXITCODE2_PATH)
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_HOOKS_EXITCODE2_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_HOOKS_EXITCODE2_PATH)

	err := runTerragruntCommand(t, fmt.Sprintf("terragrunt plan -detailed-exitcode --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), os.Stdout, os.Stderr)

	_, exception := ioutil.ReadFile(rootPath + "/test.out")
	assert.NoError(t, exception)
	assert.Contains(t, err.Error(), "There are changes in the plan")
}
