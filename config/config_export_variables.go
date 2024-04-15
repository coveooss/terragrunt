package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/terragrunt/v2/util"
	hclwrite "github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform/configs"
	"gopkg.in/yaml.v2"
)

// ExportVariablesConfig represents a path and format where variables known to Terragrunt should be exported
type ExportVariablesConfig struct {
	Path        string `hcl:"path"`
	Format      string `hcl:"format,optional"`
	SkipOnError bool   `hcl:"skip_on_error,optional"`

	exportConfig bool
}

// ExportVariables saves variables to paths defined in the export_variables blocks
func (conf *TerragruntConfig) ExportVariables(existingTerraformVariables map[string]*configs.Variable, folders ...string) (err error) {
	acceptedFormats := []string{"yml", "yaml", "tfvars", "hcl", "json", "tf"}

	exportStatements := append([]ExportVariablesConfig{}, conf.ExportVariablesConfigs...)
	for _, exportStatement := range conf.ExportConfigConfigs {
		exportStatement.exportConfig = true
		exportStatements = append(exportStatements, exportStatement)
	}

	for _, exportStatement := range exportStatements {
		handleError := func(err error) error {
			if err == nil {
				return nil
			}
			message := fmt.Errorf("caught an error while handling the following export statement: Path: %v, is config: %t\n%v", exportStatement.Path, exportStatement.exportConfig, err)
			if exportStatement.SkipOnError {
				conf.options.Logger.Error(message)
				return nil
			}
			return message
		}

		var variables collections.IDictionary
		if exportStatement.exportConfig {
			if variables, err = conf.AsDictionary(); handleError(err) != nil {
				return fmt.Errorf("couldn't fetch the config as a dictionary: %v", err)
			}
		} else {
			variables = conf.options.GetContext()
		}

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
				content, err = marshalHcl2Attributes(variables.AsMap())
			case "json":
				content, err = json.MarshalIndent(variables, "", "  ")
			case "tf":
				content, err = marshalTerraformVariables(existingTerraformVariables, variables.AsMap())
			default:
				err = fmt.Errorf("unknown export_variables format: %s, Accepted formats: %v", exportStatement.Format, acceptedFormats)
			}
			if handleError(err) != nil {
				return fmt.Errorf("marshalling error: %v", err)
			}
			if len(content) > 0 && content[len(content)-1] != '\n' {
				content = append(content, '\n')
			}
			if err = ioutil.WriteFile(writePath, content, 0644); handleError(err) != nil {
				return fmt.Errorf("file write error: %v", err)
			}
		}
	}
	return
}

// marshalHcl2Attributes marshals the given variables as HCL2 attributes (not blocks)
func marshalHcl2Attributes(variables map[string]interface{}) ([]byte, error) {
	file := hclwrite.NewEmptyFile()
	for key, valueInterface := range variables {
		value, err := util.ToCtyValue(valueInterface)
		if err != nil {
			return nil, err
		}
		file.Body().SetAttributeValue(key, *value)
	}
	return file.Bytes(), nil
}

// marshalTerraformVariables marshals the given variables as Terraform variable blocks
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
