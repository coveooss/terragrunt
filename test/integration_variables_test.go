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
	t.Parallel()
	type test struct {
		project        string
		args           string
		expectedOutput []string
	}
	tests := []test{
		{
			project:        "fixture-variables/basic-file",
			expectedOutput: []string{"example = 123"},
		},
		// Hook prints out the content of the subfolder. Shouldn't contain test.tf
		{
			project:        "fixture-variables/basic",
			expectedOutput: []string{"sub folder content: zzz_unrelated.yaml"},
		},
		{
			project:        "fixture-variables/glob-file",
			expectedOutput: []string{"example = json1-yaml1-json2-yaml2"},
		},
		{
			project:        "fixture-variables/no-tf-variables",
			args:           "--terragrunt-apply-template",
			expectedOutput: []string{"example = 123456789"},
		},
		{
			project:        "fixture-variables/overwrite",
			expectedOutput: []string{"example = 456"},
		},
		{
			project:        "fixture-variables/overwrite-with-file",
			expectedOutput: []string{"example = stay the same -> Cool value, sis"},
		},
		{
			project:        "fixture-variables/nested",
			expectedOutput: []string{"example = 123-456"},
		},
		{
			project:        "fixture-variables/different-types",
			expectedOutput: []string{"example = first-hello"},
		},
		{
			project:        "fixture-variables/load-tf-variables",
			expectedOutput: []string{"example = hello1-hello2-hello1-hello2"},
		},
		{
			project:        "fixture-variables/map",
			expectedOutput: []string{"example = 1-2-1-2-1-2"},
			args:           "--terragrunt-apply-template",
		},
		{
			project:        "fixture-variables/source",
			expectedOutput: []string{"example = 123456"},
		},
		{
			project:        "fixture-variables/module-inline",
			expectedOutput: []string{"example = 123"},
		},
		{
			project:        "fixture-variables/module-external-folder",
			expectedOutput: []string{"example = 123"},
			args:           "--terragrunt-apply-template",
		},
		{
			project:        "fixture-variables/multiple-nested",
			expectedOutput: []string{"nested = 123"},
			args:           "--terragrunt-apply-template",
		},
		{
			project:        "fixture-variables/templating-in-file",
			expectedOutput: []string{"example = 123"},
			args:           "--terragrunt-apply-template",
		},
		// This is the same as `templating-in-file`, however `no_templating_in_files` is passed to the `import_variables` statement, so the template is not resolved
		{
			project:        "fixture-variables/no-templating-in-file",
			expectedOutput: []string{"example = @template"},
			args:           "--terragrunt-apply-template",
		},
		{
			project:        "fixture-variables/overridden-explicit-variable",
			expectedOutput: []string{"example = us-west-2"},
			args:           "--terragrunt-apply-template -var region=us-east-1",
		},
	}
	for _, test := range tests {
		tt := test // tt must be unique see https://github.com/golang/go/issues/16586
		t.Run(tt.project, func(t *testing.T) {
			t.Parallel()
			tmpEnvPath := copyEnvironment(t, tt.project)
			defer os.RemoveAll(tmpEnvPath)
			rootPath := util.JoinPath(tmpEnvPath, tt.project)

			var stdout, stderr bytes.Buffer
			runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s %s", rootPath, tt.args), &stdout, &stderr)
			for _, expectedOutput := range tt.expectedOutput {
				assert.Contains(t, stdout.String(), expectedOutput)
			}
		})
	}
}
