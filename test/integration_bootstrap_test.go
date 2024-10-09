package test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coveooss/terragrunt/v2/util"
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
			bootstrap:      []string{absoluteTestPath + "/simple/a.hcl", absoluteTestPath + "/simple/b.hcl"},
			expectedOutput: "applyHook", // This is output by the hook
		},
		{
			name:           "Simple pre-bootstrap (adding variables)",
			project:        "fixture-bootstrap/preboot",
			preboot:        []string{absoluteTestPath + "/preboot/variables.json", absoluteTestPath + "/preboot/variables.yml"},
			expectedOutput: "my_value my_value2 my_value3", // This is output by the hook
		},
		{
			name:           "Simple pre-bootstrap with a file prepended with file:// (Testing accepted values)",
			project:        "fixture-bootstrap/preboot",
			preboot:        []string{"file://" + absoluteTestPath + "/preboot/variables.json", absoluteTestPath + "/preboot/variables.yml"},
			expectedOutput: "my_value my_value2 my_value3", // This is output by the hook
		},
		{
			name:           "Complex project with a bootstrap that defines the Terraform source",
			project:        "fixture-bootstrap/refer-to-other-source",
			terragruntPath: "/terragrunt_files/my_project",
			bootstrap:      []string{absoluteTestPath + "/_external_dir/refer-to-other-source.hcl"},
			expectedOutput: "test output",
		},
		{
			name:           "Test go templating in a template file",
			project:        "fixture-bootstrap/templating-in-bootstrap",
			bootstrap:      []string{absoluteTestPath + "/_external_dir/templating-in-bootstrap.hcl"},
			expectedOutput: "test variable", // This is output by the hook
		},
		{
			name:           "Test go templating with terragrunt functions in a template file",
			project:        "fixture-bootstrap/terragrunt-function-in-bootstrap",
			bootstrap:      []string{absoluteTestPath + "/_external_dir/terragrunt-function-in-bootstrap.hcl"},
			expectedOutput: "default_env_value", // This is output by the hook
		},
		{
			name:           "Complex case where a pre-bootstrap file defines variables and another creates new variables from templating",
			project:        "fixture-bootstrap/chain-preboot-configs",
			preboot:        []string{absoluteTestPath + "/chain-preboot-configs/preboot.yaml", absoluteTestPath + "/chain-preboot-configs/variables.json"},
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

			args := ""
			if len(tt.bootstrap) > 0 {
				args += " --terragrunt-boot-configs " + strings.Join(tt.bootstrap, ",")
			}
			if len(tt.preboot) > 0 {
				args += " --terragrunt-pre-boot-configs " + strings.Join(tt.preboot, ",")
			}

			var stdout, stderr bytes.Buffer
			command := fmt.Sprintf("terragrunt apply --terragrunt-apply-template --terragrunt-non-interactive --terragrunt-working-dir %s%s %s", rootPath, tt.terragruntPath, args)
			runTerragruntRedirectOutput(t, command, &stdout, &stderr)
			assert.Contains(t, stdout.String(), tt.expectedOutput, "Received %s", stdout.String())
		})
	}
}
