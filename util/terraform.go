package util

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform/configs"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/json"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/yaml"
	"github.com/coveooss/multilogger/errors"
	"github.com/sirupsen/logrus"
)

// LoadDefaultValues returns a map of the variables defined in the tfvars file
func LoadDefaultValues(folder string) (importedVariables map[string]interface{}, allVariables map[string]*configs.Variable, err error) {
	var terraformConfig *configs.Module
	parser := configs.NewParser(nil)
	if terraformConfig, err = parser.LoadConfigDir(folder); err != nil && err.(hcl.Diagnostics).HasErrors() {
		err = fmt.Errorf("caught errors while trying to load default variable values from %s:\n%w", folder, errors.Array(err.(hcl.Diagnostics).Errs()))
		return
	}
	importedVariables, err = getTerraformVariableValues(terraformConfig, false)
	allVariables = terraformConfig.Variables
	return
}

// LoadVariablesFromHcl parses HCL content to get a map of the attributes
func LoadVariablesFromHcl(filename string, bytes []byte) (map[string]interface{}, error) {
	hclFile, diag := hclparse.NewParser().ParseHCL([]byte(bytes), filename)
	if diag != nil && diag.HasErrors() {
		return nil, fmt.Errorf("caught an error while parsing HCL from %s: %w", filename, diag)
	}

	attributes, diag := hclFile.Body.JustAttributes()
	if diag != nil && diag.HasErrors() {
		return nil, fmt.Errorf("caught an error while getting content from %s: %w", filename, diag)
	}

	result := make(map[string]interface{})

	for key, attribute := range attributes {
		ctyValue, diag := attribute.Expr.Value(nil)
		if diag != nil && diag.HasErrors() {
			return nil, fmt.Errorf("caught an error while reading attribute %s from %s: %w", key, filename, diag)
		}
		var value interface{}
		if err := FromCtyValue(ctyValue, &value); err != nil {
			return nil, err
		}
		// All numbers are floats in HCL2, an integer is just a validation over that
		// However, this is not so for Go itself which distinguishes between float and int
		// We can therefore cast floats as ints if they are absolute
		// The opposite case (defining an absolute float that will be cast into a int) will break but it is far less common
		if floatValue, isFloat := value.(float64); isFloat && floatValue == float64(int(floatValue)) {
			value = int(floatValue)
		}
		result[key] = value
	}
	return result, nil
}

// LoadVariablesFromFile returns a map of the variables defined in the tfvars file
func LoadVariablesFromFile(path, cwd string, applyTemplate bool, context ...interface{}) (map[string]interface{}, error) {
	if filepath.Ext(path) == ".tf" {
		parser := configs.NewParser(nil)
		if terraformConfig, err := parser.LoadConfigFile(path); err == nil {
			return getTerraformVariableValues(terraformConfig, false)
		}
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result, err := LoadVariablesFromSource(string(bytes), path, cwd, applyTemplate, context...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// LoadVariables returns a map of the variables defined in the content provider
func LoadVariables(content string, cwd string, applyTemplate bool, context ...interface{}) (map[string]interface{}, error) {
	return LoadVariablesFromSource(content, "Terragrunt content", cwd, applyTemplate, context...)
}

// LoadVariablesFromSource returns a map of the variables defined in the content provider
func LoadVariablesFromSource(content, fileName, cwd string, applyTemplate bool, context ...interface{}) (result map[string]interface{}, err error) {
	result = make(map[string]interface{})
	if applyTemplate && template.IsCode(content) {
		var t *template.Template
		switch len(context) {
		case 0:
			break
		case 1:
			t, err = template.NewTemplate(cwd, context[0], "", nil)
		default:
			t, err = template.NewTemplate(cwd, context, "", nil)
		}
		if err != nil {
			return
		}

		if t != nil {
			if err := template.TemplateLog.SetDefaultConsoleHookLevel(logrus.InfoLevel); err != nil {
				return nil, fmt.Errorf("unable to set logging level for templates: %w", err)
			}
			if modifiedContent, err := t.ProcessContent(content, fileName); err != nil {
				// In case of error, we simply issue a warning and continue with the original content
				template.TemplateLog.Warning(err)
			} else {
				content = modifiedContent
			}
		}
	}

	if strings.HasSuffix(fileName, ".json") {
		err = json.Unmarshal([]byte(content), &result)
		return
	}
	if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
		err = yaml.Unmarshal([]byte(content), &result)
		return
	}

	// We first try to read using hcl parser
	result, err = LoadVariablesFromHcl(fileName, []byte(content))
	if err != nil {
		// If there is an error with try with a multi language parser
		result = make(map[string]interface{})
		if err2 := collections.ConvertData(content, &result); err2 == nil {
			// We succeeded, hooray!
			err = nil
		} else {
			// We add the file name to the error
			err = fmt.Errorf("error in %s:%s", fileName, strings.TrimPrefix(err.Error(), "At "))
		}
	}
	return
}

func getTerraformVariableValues(terraformConfig interface{}, includeNil bool) (map[string]interface{}, error) {
	variables := []*configs.Variable{}
	switch source := terraformConfig.(type) {
	case *configs.Module:
		for _, variable := range source.Variables {
			variables = append(variables, variable)
		}
	case *configs.File:
		variables = source.Variables
	}
	variablesMap := map[string]interface{}{}
	for _, variable := range variables {
		if includeNil || !variable.Default.IsNull() {
			var value interface{}
			if err := FromCtyValue(variable.Default, &value); err != nil {
				return nil, err
			}
			if !template.IsCode(fmt.Sprint(value)) {
				// The default value contains gotemplate code, we don't want to make it available
				// to gotemplate and we let terraform code initialize the value
				variablesMap[variable.Name] = value
			}
		}
	}
	return variablesMap, nil
}
