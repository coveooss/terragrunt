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

func TestTerragruntBeforeHook(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-hooks/before-only"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))

	_, exception := ioutil.ReadFile(rootPath + "/file.out")

	assert.NoError(t, exception)
}

func TestTerragruntAfterHook(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-hooks/after-only"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))

	_, exception := ioutil.ReadFile(rootPath + "/file.out")

	assert.NoError(t, exception)
}

func TestTerragruntBeforeAndAfterHook(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-hooks/before-and-after"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

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

	const testPath = "fixture-hooks/before-and-after-merge"
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)
	childPath := util.JoinPath(rootPath, TEST_FIXTURE_INCLUDE_CHILD_REL_PATH)
	cleanupTerraformFolder(t, childPath)

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))
	t.Logf("bucketName: %s", s3BucketName)
	defer deleteS3Bucket(t, TERRAFORM_REMOTE_STATE_S3_REGION, s3BucketName)

	tmpTerragruntConfigPath := createTmpTerragruntConfigWithParentAndChild(t, testPath, TEST_FIXTURE_INCLUDE_CHILD_REL_PATH, s3BucketName, config.DefaultTerragruntConfigPath, config.DefaultTerragruntConfigPath)

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

	const testPath = "fixture-hooks/interpolations"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

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

	const testPath = "fixture-hooks/exitcode-1"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

	err := runTerragruntCommand(t, fmt.Sprintf("terragrunt plan -detailed-exitcode --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), os.Stdout, os.Stderr)

	_, exception := ioutil.ReadFile(rootPath + "/test.out")
	assert.Error(t, exception)
	assert.Contains(t, err.Error(), "Error while executing hooks(post_hook_1): exit status 1")
}

func TestTerragruntHookExitCode2(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-hooks/exitcode-2"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

	err := runTerragruntCommand(t, fmt.Sprintf("terragrunt plan -detailed-exitcode --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), os.Stdout, os.Stderr)

	_, exception := ioutil.ReadFile(rootPath + "/test.out")
	assert.NoError(t, exception)
	assert.Contains(t, err.Error(), "There are changes in the plan")
}

func TestTerragruntHookExitCode2InPreHook(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-hooks/exitcode-2-pre"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

	err := runTerragruntCommand(t, fmt.Sprintf("terragrunt plan -detailed-exitcode --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), os.Stdout, os.Stderr)

	_, exception := ioutil.ReadFile(rootPath + "/test2.out")
	assert.NoError(t, exception)
	assert.Contains(t, err.Error(), "There are changes in the plan")
}

func TestTerragruntHookExitCode2PlanAll(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-hooks/exitcode-2"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

	err := runTerragruntCommand(t, fmt.Sprintf("terragrunt plan-all -detailed-exitcode --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), os.Stdout, os.Stderr)

	_, exception := ioutil.ReadFile(rootPath + "/test.out")
	assert.NoError(t, exception)
	assert.Contains(t, err.Error(), "There are changes in the plan")
}

func TestTerragruntHookWithEnvVars(t *testing.T) {
	for i := 1; i <= 2; i++ {
		withEnv("PATH", func() {
			const testPath = "fixture-hooks/with-envvars"
			cleanupTerraformFolder(t, testPath)
			tmpEnvPath := copyEnvironment(t, testPath)
			rootPath := util.JoinPath(tmpEnvPath, testPath)

			var stdout, stderr bytes.Buffer
			runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt cmd%d --terragrunt-non-interactive --terragrunt-working-dir %s", i, rootPath), &stdout, &stderr)
			content, err := ioutil.ReadFile(util.JoinPath(rootPath, fmt.Sprintf("result%d", i)))
			assert.NoError(t, err, "Reading result%d", i)
			assert.Equal(t, string(content), stdout.String(), "Comparing result %d", i)
		})
	}
}

func TestTerragruntHookOverwrite(t *testing.T) {
	const testPath = "fixture-hooks/overwrite"
	cleanupTerraformFolder(t, testPath)
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)

	var stdout bytes.Buffer
	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt cmd --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, os.Stderr)
	content, err := ioutil.ReadFile(util.JoinPath(rootPath, "result"))
	assert.NoError(t, err, "Reading result")
	assert.Equal(t, string(content), stdout.String(), "Comparing result")
}
