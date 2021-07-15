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

// ExportVariables is a configuration that is used to export variables to a file
type ExportVariables struct {
	ExportVariablesConfig `hcl:",squash"`
	ImportIntoModules     bool `hcl:"import_into_modules,optional"`
}

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.export_variables.go gen TypeName=ExportVariables
func (list ExportVariablesList) argName() string      { return "export_variables" }
func (list ExportVariablesList) mergeMode() mergeMode { return mergeModeAppend }

// Export export all required variables to the specified files
func (list ExportVariablesList) Export(existingTerraformVariables map[string]*configs.Variable, folder string, isRootFolder bool) error {
	for i := range list.Enabled() {
		if !isRootFolder && !list[i].ImportIntoModules {
			continue
		}
		if err := list[i].Export(existingTerraformVariables, folder, list[i].options().GetContext()); err != nil {
			return err
		}
	}
	return nil
}

// ExportConfig is a configuration that is used to export the current configuration to a file
type ExportConfig struct {
	ExportVariablesConfig `hcl:",squash"`
}

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.export_config.go gen TypeName=ExportConfig
func (list ExportConfigList) argName() string      { return "export_config" }
func (list ExportConfigList) mergeMode() mergeMode { return mergeModeAppend }

// Export save the current configuration to all specified files
func (list ExportConfigList) Export() error {
	if len(list) == 0 {
		return nil
	}
	config, err := list[0].config().AsDictionary()
	if err != nil {
		return fmt.Errorf("couldn't fetch the config as a dictionary: %v", err)
	}
	for i := range list.Enabled() {
		if err := list[i].Export(nil, "", config); err != nil {
			return err
		}
	}
	return nil
}

// ----------------------- Commont implementation -----------------------

// ExportVariablesConfig represents a path and format where variables known to Terragrunt should be exported
type ExportVariablesConfig struct {
	TerragruntExtension `hcl:",squash"`

	Path        string `hcl:"path"`
	Format      string `hcl:"format,optional"`
	SkipOnError bool   `hcl:"skip_on_error,optional"`
}

// Export saves the supplied variables to paths defined in the export_variables blocks
func (e *ExportVariablesConfig) Export(existingTerraformVariables map[string]*configs.Variable, folder string, variables collections.IDictionary) (err error) {
	acceptedFormats := []string{"yml", "yaml", "tfvars", "hcl", "json", "tf"}

	handleError := func(err error) error {
		if err == nil {
			return nil
		}
		message := fmt.Errorf("caught an error while handling the following export statement: %s Path: %v\n%v", e.itemType(), e.Path, err)
		if e.SkipOnError {
			e.logger().Error(message)
			return nil
		}
		return message
	}

	e.Format = strings.TrimSpace(e.Format)
	if e.Format == "" {
		e.Format = strings.Trim(filepath.Ext(e.Path), ". ")
	}
	if e.Format == "" {
		return fmt.Errorf("an export_variables statement must either define an export format or a file extension matching one of the export formats. Given path: %s, Given format: %s, Accepted formats: %v", e.Path, e.Format, acceptedFormats)
	}

	writePath := filepath.Join(folder, e.Path)
	e.logger().Debug("Saving variables into ", writePath)
	var content []byte
	switch e.Format {
	case "yml", "yaml":
		content, err = yaml.Marshal(variables)
	case "tfvars", "hcl":
		content, err = marshalHcl2Attributes(variables.AsMap())
	case "json":
		content, err = json.MarshalIndent(variables, "", "  ")
	case "tf":
		content, err = marshalTerraformVariables(existingTerraformVariables, variables.AsMap())
	default:
		err = fmt.Errorf("unknown export_variables format: %s, Accepted formats: %v", e.Format, acceptedFormats)
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
