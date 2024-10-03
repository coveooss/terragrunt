package test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/coveooss/terragrunt/v2/util"
	"github.com/stretchr/testify/assert"
)

func TestTerragruntGoTemplate(t *testing.T) {
	t.Parallel()
	type test struct {
		project        string
		args           string
		expectedOutput []string
	}
	tests := []test{
		// Test that loading default variables works even if the terraform original source is not compliant
		{
			project: "fixture-gotemplate",
			args:    "--terragrunt-apply-template --terragrunt-logging-level DEBUG",
			expectedOutput: []string{
				`^This_is_an_output = "ok"$`,
				`^test1 = "I am test 1"$`,
				`^test2 = "I am test 2 \(overridden\)"$`,
				`^json1 = "I am json 1"$`,
				`^json2 = "I am json 2 \(overridden\)"$`,
			},
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
			runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply -no-color --terragrunt-non-interactive --terragrunt-working-dir %s %s", rootPath, tt.args), &stdout, &stderr)
			for _, expectedOutput := range tt.expectedOutput {
				assert.Regexp(t, fmt.Sprintf(`(?m).*%s.*`, expectedOutput), stdout.String())
			}
		})
	}
}
