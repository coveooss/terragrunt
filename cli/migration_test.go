package cli

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/lithammer/dedent"
	"github.com/stretchr/testify/assert"
)

func TestMigrateConfigurationFile(t *testing.T) {
	t.Parallel()

	type vars = map[string]interface{}
	tests := []struct {
		name     string
		initial  string
		expected string
	}{
		{
			name:     "Empty file",
			initial:  "",
			expected: "",
		},
		{
			name:     "Empty config",
			initial:  "terragrunt {}",
			expected: "",
		},
		{
			name: "Empty config multiline",
			initial: `terragrunt {

			}`,
			expected: "",
		},
		{
			name: "Empty config multiline with equal",
			initial: `terragrunt = {

			}`,
			expected: "",
		},
		{
			name: "Simple case",
			initial: `
			terragrunt = {
				post_hook "post_hook_1" {
					on_commands = ["apply", "plan"]
					command     = "exit 2"
				}

				post_hook "post_hook_2" {
					on_commands = ["apply", "plan"]
					command     = "touch test.out"
				}
			}`,
			expected: `
			post_hook "post_hook_1" {
				on_commands = ["apply", "plan"]
				command     = "exit 2"
			}

			post_hook "post_hook_2" {
				on_commands = ["apply", "plan"]
				command     = "touch test.out"
			}`,
		},
		{
			name: "With Variables",
			initial: `
			before = {
				variable = {
					inner = "text"
				}
			}

			terragrunt{
				import_variables "test" {
					required_var_files = [
						"vars.json",
					]

					output_variables_file = "test.tf"
				}
			}

			after = [
				{
					inner = "text"
				},
				{
					inner = "text2"
				}
			]`,
			expected: `
			import_variables "test" {
				required_var_files = [
					"vars.json",
				]

				output_variables_file = "test.tf"
			}
			inputs = {
				after = [
					{
						inner = "text"
					},
					{
						inner = "text2"
					},
				]

				before = {
					variable = {
						inner = "text"
					}
				}
			}`,
		},
		{
			name: "",
			initial: `
			project = "spinnaker"

			namespace = "spinnaker"
			
			source_path = "../modules"
			
			terragrunt = {
			  assume_role = "${var.deploy_role_ops}"
			}
			`,
			expected: `
			assume_role = "${var.deploy_role_ops}"
			inputs = {
				project = "spinnaker"
				namespace = "spinnaker"
				source_path = "../modules"
			}
			`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			configDir, _ := ioutil.TempDir("", "TerragruntTestMigrateConfigurationFile")
			configFile := path.Join(configDir, "terraform.tfvars")
			resultFile := path.Join(configDir, config.DefaultTerragruntConfigPath)
			defer os.RemoveAll(configDir)

			// Initial config
			ioutil.WriteFile(configFile, []byte(dedent.Dedent(tt.initial)), 0777)

			// Migrate (No error)
			assert.Nil(t, migrateConfigurationFile(configFile, "terraform.tfvars"))

			// Verify content
			result, _ := ioutil.ReadFile(resultFile)
			var (
				expectedParsed interface{}
				resultParsed   interface{}
			)
			hcl.Unmarshal([]byte(tt.expected), &expectedParsed)
			hcl.Unmarshal(result, &resultParsed)
			assert.Equal(t, expectedParsed, resultParsed)

			// Verify that the old config file does not exist
			_, err := os.Stat(configFile)
			assert.Truef(t, os.IsNotExist(err), "%s should not exist", configFile)
		})
	}

}
