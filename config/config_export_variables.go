package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/hashicorp/terraform/configs"
	"gopkg.in/yaml.v2"
)

// ExportVariablesConfig represents a path and format where variables known to Terragrunt should be exported
type ExportVariablesConfig struct {
	Path   string `hcl:"path"`
	Format string `hcl:"format,optional"`
}

// ExportVariables saves variables to paths defined in the export_variables blocks
func (conf *TerragruntConfig) ExportVariables(existingTerraformVariables map[string]*configs.Variable, folders ...string) (err error) {
	variables := conf.options.GetContext()

	acceptedFormats := []string{"yml", "yaml", "tfvars", "hcl", "json", "tf"}

	for _, exportStatement := range conf.ExportVariablesConfigs {
		exportStatement.Format = strings.TrimSpace(exportStatement.Format)
		if exportStatement.Format == "" {
			exportStatement.Format = strings.Trim(filepath.Ext(exportStatement.Path), ". ")
		}
		if exportStatement.Format == "" {
			return fmt.Errorf("an export_variables statement must either define an export format or a file extension matching one of the export formats. Given path: %s, Given format: %s, Accepted formats: %v", exportStatement.Path, exportStatement.Format, acceptedFormats)
		}

		for _, folder := range folders {
			writePath := filepath.Join(folder, exportStatement.Path)
			conf.options.Logger.Debug("Saving variables into ", writePath)
			var content []byte
			switch exportStatement.Format {
			case "yml", "yaml":
				content, err = yaml.Marshal(variables)
			case "tfvars", "hcl":
				content, err = hcl.MarshalTFVarsIndent(variables, "", "  ")
			case "json":
				content, err = json.MarshalIndent(variables, "", "  ")
			case "tf":
				content, err = marshalTerraformVariables(existingTerraformVariables, variables.AsMap())
			default:
				err = fmt.Errorf("unknown export_variables format: %s, Accepted formats: %v", exportStatement.Format, acceptedFormats)
			}
			if err != nil {
				return
			}
			if len(content) > 0 && content[len(content)-1] != '\n' {
				content = append(content, '\n')
			}
			err = ioutil.WriteFile(writePath, content, 0644)
		}
	}
	return
}

func marshalTerraformVariables(existingTerraformVariables map[string]*configs.Variable, variables map[string]interface{}) ([]byte, error) {
	lines := []string{}
	for key, value := range variables {
		if _, ok := existingTerraformVariables[key]; ok {
			continue
		}
		lines = append(lines, fmt.Sprintf(`variable "%s" {`, key))
		if value != nil {
			variableContent, err := hcl.MarshalTFVarsIndent(map[string]interface{}{"default": value}, "  ", "  ")
			if err != nil {
				return nil, err
			}
			lines = append(lines, string(variableContent))
		}
		lines = append(lines, "}", "")
	}
	return []byte(strings.Join(lines, "\n")), nil
}
