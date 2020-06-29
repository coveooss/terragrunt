package test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/coveooss/terragrunt/v2/util"
	"github.com/stretchr/testify/assert"
)

func TestTerragruntImportFiles(t *testing.T) {
	tests := []struct {
		project        string
		expectedOutput string
	}{
		{
			project:        "fixture-import-files/basic",
			expectedOutput: "example = 123",
		},
		{
			project:        "fixture-import-files/bad-source",
			expectedOutput: "example = 123",
		},
		{
			project:        "fixture-import-files/overwrite",
			expectedOutput: "example = 456",
		},
	}
	for _, tt := range tests {
		t.Run(tt.project, func(t *testing.T) {
			tmpEnvPath := copyEnvironment(t, tt.project)
			defer os.RemoveAll(tmpEnvPath)
			rootPath := util.JoinPath(tmpEnvPath, tt.project)

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
