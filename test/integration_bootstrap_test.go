package test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
)

func TestTerragruntBootstrap(t *testing.T) {
	t.Parallel()
	absoluteTestPath, _ := filepath.Abs("../test/fixture-bootstrap")
	tests := []struct {
		name           string
		project        string
		terragruntPath string
		preboot        []string
		bootstrap      []string
		expectedOutput string
	}{
		{
			name:           "Simple bootstrap (adding hooks)",
			project:        "fixture-bootstrap/simple",
			bootstrap:      []string{absoluteTestPath + "/simple/a.tfvars", absoluteTestPath + "/simple/b.tfvars"},
			expectedOutput: "applyHook", // This is output by the hook
		},
		{
			name:           "Simple pre-bootstrap (adding variables)",
			project:        "fixture-bootstrap/preboot",
			preboot:        []string{absoluteTestPath + "/preboot/variables.json", absoluteTestPath + "/preboot/variables.yml"},
			expectedOutput: "my_value my_value2 my_value3", // This is output by the hook
		},
		{
			name:           "Simple pre-bootstrap with a file prependend with file:// (Testing accepted values)",
			project:        "fixture-bootstrap/preboot",
			preboot:        []string{"file://" + absoluteTestPath + "/preboot/variables.json", absoluteTestPath + "/preboot/variables.yml"},
			expectedOutput: "my_value my_value2 my_value3", // This is output by the hook
		},
		{
			name:           "Complex project with a bootstrap that defines the Terraform source",
			project:        "fixture-bootstrap/refer-to-other-source",
			terragruntPath: "/terragrunt_files/my_project",
			bootstrap:      []string{absoluteTestPath + "/_external_dir/refer-to-other-source.tfvars"},
			expectedOutput: "test output",
		},
		{
			name:           "Test gotemplating in a template file",
			project:        "fixture-bootstrap/templating-in-bootstrap",
			bootstrap:      []string{absoluteTestPath + "/_external_dir/templating-in-bootstrap.tfvars"},
			expectedOutput: "test variable", // This is output by the hook
		},
		{
			name:           "Complex case where a pre-bootstrap file defines variables and another creates new variables from templating",
			project:        "fixture-bootstrap/chain-preboot-configs",
			preboot:        []string{absoluteTestPath + "/chain-preboot-configs/preboot.hcl", absoluteTestPath + "/chain-preboot-configs/variables.json"},
			expectedOutput: "value1 value2", // This is output by the hook
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpEnvPath := copyEnvironment(t, tt.project)
			defer os.RemoveAll(tmpEnvPath)
			rootPath := util.JoinPath(tmpEnvPath, tt.project)

			var (
				stdout bytes.Buffer
				stderr bytes.Buffer
			)

			args := ""
			if len(tt.bootstrap) > 0 {
				args += " --terragrunt-boot-configs " + strings.Join(tt.bootstrap, ",")
			}
			if len(tt.preboot) > 0 {
				args += " --terragrunt-pre-boot-configs " + strings.Join(tt.preboot, ",")
			}

			runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-apply-template --terragrunt-non-interactive --terragrunt-working-dir %s%s %s", rootPath, tt.terragruntPath, args), &stdout, &stderr)
			output := stdout.String()
			assert.Contains(t, output, tt.expectedOutput)

		})
	}
}
