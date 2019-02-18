package test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
)

func TestTerragruntImportVariables(t *testing.T) {
	tests := []struct {
		project        string
		envVariables   map[string]string
		expectedOutput string
	}{
		{
			project:        "fixture-variables/basic-file",
			expectedOutput: "example = 123",
		},
		// Hook prints out the content of the subfolder. Shouldn't contain test.tf
		{
			project:        "fixture-variables/basic",
			expectedOutput: "sub folder content:\nzzz_unrelated.yaml\n",
		},
		{
			project:        "fixture-variables/glob-file",
			expectedOutput: "example = json1-yaml1-json2-yaml2",
		},
		{
			project:        "fixture-variables/no-tf-variables",
			envVariables:   map[string]string{"TERRAGRUNT_TEMPLATE": "true"},
			expectedOutput: "example = 123456789",
		},
		{
			project:        "fixture-variables/flatten",
			expectedOutput: "example = 1-2-hello-twolevels",
		},
		{
			project:        "fixture-variables/flatten-overwrite",
			expectedOutput: "example = 1-3-4-twolevels",
		},
		{
			project:        "fixture-variables/overwrite",
			expectedOutput: "example = 456",
		},
		{
			project:        "fixture-variables/overwrite-with-file",
			expectedOutput: "example = 456",
		},
		{
			project:        "fixture-variables/placeholder-var",
			expectedOutput: "example = 123",
		},
		{
			project:        "fixture-variables/substitute",
			expectedOutput: "example = hello-hello2",
		},
		{
			project:        "fixture-variables/nested",
			expectedOutput: "example = 123-456",
		},
		{
			project:        "fixture-variables/different-types",
			expectedOutput: "example = first-hello",
		},
		{
			project:        "fixture-variables/load-tf-variables",
			expectedOutput: "example = hello1-hello2-hello1-hello2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.project, func(t *testing.T) {
			tmpEnvPath := copyEnvironment(t, tt.project)
			defer os.RemoveAll(tmpEnvPath)
			rootPath := util.JoinPath(tmpEnvPath, tt.project)

			for key, value := range tt.envVariables {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			var (
				stdout bytes.Buffer
				stderr bytes.Buffer
			)

			runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr)
			output := stdout.String()
			assert.Contains(t, output, tt.expectedOutput)
		})
	}
}
