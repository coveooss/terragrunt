package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/gruntwork-io/terragrunt/util"
	hclwrite "github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform/configs"
	"gopkg.in/yaml.v2"
)

type ExportVariables struct {
	Path   string `hcl:"path"`
	Format string `hcl:"format,optional"`
}

// SaveVariables saves variables in paths defined in the export_variables blocks
func (conf *TerragruntConfig) SaveVariables(existingTerraformVariables map[string]*configs.Variable, folders ...string) (err error) {
	variables := conf.options.GetContext()

	for _, exportStatement := range conf.ExportVariables {
		exportStatement.Format = strings.TrimSpace(exportStatement.Format)
		if exportStatement.Format == "" {
			exportStatement.Format = strings.Trim(filepath.Ext(exportStatement.Path), ". ")
		}
		if exportStatement.Format == "" {
			return fmt.Errorf("an export_variables statement must either define an export format or a significant export path file extension. Given path: %s, Given format: %s", exportStatement.Path, exportStatement.Format)
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
				err = fmt.Errorf("unknown export_variables format: %s", exportStatement.Format)
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
	file := hclwrite.NewEmptyFile()
	for key, value := range variables {
		if _, ok := existingTerraformVariables[key]; ok {
			continue
		}
		block := file.Body().AppendNewBlock("variable", []string{key})
		ctyValue, err := util.ToCtyValue(value)
		if err != nil {
			return nil, err
		}
		block.Body().SetAttributeValue("default", *ctyValue)
	}
	return file.Bytes(), nil
}
