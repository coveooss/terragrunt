package test

import (
	"bytes"
	"fmt"
	"os"
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
	TEST_FIXTURE_VARIABLES_GLOB_FILE_PATH           = "fixture-variables/glob-file"
	TEST_FIXTURE_VARIABLES_NO_TF_VARIABLES_PATH     = "fixture-variables/no-tf-variables"
	TEST_FIXTURE_VARIABLES_OVERWRITE_PATH           = "fixture-variables/overwrite"
	TEST_FIXTURE_VARIABLES_OVERWRITE_WITH_FILE_PATH = "fixture-variables/overwrite-with-file"
	TEST_FIXTURE_VARIABLES_PLACEHOLDER_VAR_PATH     = "fixture-variables/placeholder-var"
	TEST_FIXTURE_VARIABLES_SUBSTITUTE_PATH          = "fixture-variables/substitute"
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
}

func TestTerragruntImportVariablesGlobFile(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_GLOB_FILE_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_GLOB_FILE_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = json1-yaml1-json2-yaml2")
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
}

func TestTerragruntImportVariablesWithPlaceholder(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_PLACEHOLDER_VAR_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_PLACEHOLDER_VAR_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = 123")
}

func TestTerragruntImportVariablesWithSubstitute(t *testing.T) {
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_VARIABLES_SUBSTITUTE_PATH)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_VARIABLES_SUBSTITUTE_PATH)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
	output := stdout.String()

	assert.Contains(t, output, "example = hello-hello2")
}
