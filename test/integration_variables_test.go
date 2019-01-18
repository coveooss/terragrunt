package test

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
)

// hard-code this to match the test fixture for now
const (
	TEST_FIXTURE_VARIABLES_BASIC_PATH               = "fixture-variables/basic"
	TEST_FIXTURE_VARIABLES_BASIC_FILE_PATH          = "fixture-variables/basic-file"
	TEST_FIXTURE_VARIABLES_FLATTEN_PATH             = "fixture-variables/flatten"
	TEST_FIXTURE_VARIABLES_FLATTEN_OVERWRITE_PATH   = "fixture-variables/flatten-overwrite"
	TEST_FIXTURE_VARIABLES_NO_TF_VARIABLES_PATH     = "fixture-variables/no-tf-variables"
	TEST_FIXTURE_VARIABLES_OVERWRITE_PATH           = "fixture-variables/overwrite"
	TEST_FIXTURE_VARIABLES_OVERWRITE_WITH_FILE_PATH = "fixture-variables/overwrite-with-file"
)

func TestTerragruntImportVariablesBasic(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_BASIC_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_BASIC_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 123")
	assert.FileExists(t, path.Join(rootPath, "test.tf"))
}

func TestTerragruntImportVariablesBasicFile(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_BASIC_FILE_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_BASIC_FILE_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 123")
	assert.FileExists(t, path.Join(rootPath, "test.tf"))
}

func TestTerragruntImportVariablesFlatten(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_FLATTEN_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_FLATTEN_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 1-2-hello-twolevels")
	assert.FileExists(t, path.Join(rootPath, "test.tf"))
}

func TestTerragruntImportVariablesFlattenOverwrite(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_FLATTEN_OVERWRITE_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_FLATTEN_OVERWRITE_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 1-3-4-twolevels")
	assert.FileExists(t, path.Join(rootPath, "test.tf"))
}

func TestTerragruntImportVariablesWithoutVariablesGeneration(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_NO_TF_VARIABLES_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_NO_TF_VARIABLES_PATH)
	os.Setenv("TERRAGRUNT_TEMPLATE", "true")
	defer os.Unsetenv("TERRAGRUNT_TEMPLATE")
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 123456789")
}

func TestTerragruntImportVariablesOverwrite(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_OVERWRITE_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_OVERWRITE_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 456")
	assert.FileExists(t, path.Join(rootPath, "test.tf"))
}

func TestTerragruntImportVariablesOverwriteWithFile(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_OVERWRITE_WITH_FILE_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_OVERWRITE_WITH_FILE_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 456")
	assert.FileExists(t, path.Join(rootPath, "test.tf"))
}
