package util

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	hcl2 "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/terraform/configs"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/coveooss/gotemplate/v3/json"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/yaml"
	"github.com/sirupsen/logrus"
)

// LoadDefaultValues returns a map of the variables defined in the tfvars file
func LoadDefaultValues(folder string) (importedVariables map[string]interface{}, allVariables map[string]*configs.Variable, err error) {
	var terraformConfig *configs.Module
	parser := configs.NewParser(nil)
	if terraformConfig, err = parser.LoadConfigDir(folder); err != nil && err.(hcl2.Diagnostics).HasErrors() {
		return map[string]interface{}{}, nil, fmt.Errorf("caught error while trying to load default variable values: %v", err)
	}
	importedVariables, err = getTerraformVariableValues(terraformConfig, false)
	return importedVariables, terraformConfig.Variables, err
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
				return nil, fmt.Errorf("unable to set logging level for templates: %v", err)
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
	if err = hcl.Unmarshal([]byte(content), &result); err != nil {
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
			variablesMap[variable.Name] = value
		}
	}
	return variablesMap, nil
}
