package util

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/json"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/yaml"
	"github.com/coveooss/multilogger"
	"github.com/coveooss/multilogger/errors"
	"github.com/fatih/color"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/sirupsen/logrus"
)

type (
	cachedDefaultVariables struct {
		values map[string]interface{}
		all    map[string]*tfconfig.Variable
		err    error
	}
)

var cacheDefault sync.Map

// LoadDefaultValues returns a map of the variables defined in the tfvars file
func LoadDefaultValues(folder string, logger *multilogger.Logger, keepInCache bool) (importedVariables map[string]interface{}, allVariables map[string]*tfconfig.Variable, err error) {
	if keepInCache {
		defer func() {
			if _, exist := cacheDefault.Load(folder); !exist {
				// We put the result in cache for the next call
				cacheDefault.Store(folder, &cachedDefaultVariables{importedVariables, allVariables, err})
			}
		}()
	}

	if cached, exist := cacheDefault.Load(folder); exist {
		// If we already processed this folder, we simply return the cached values
		cached := cached.(*cachedDefaultVariables)
		return cached.values, cached.all, nil
	}

	return loadDefaultValues(folder, logger)
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
	bytes, err := os.ReadFile(path)
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

func loadDefaultValues(folder string, logger *multilogger.Logger) (map[string]interface{}, map[string]*tfconfig.Variable, error) {
	module, diags := tfconfig.LoadModule(folder)
	if diags.HasErrors() && logger != nil {
		var err errors.Array
		for _, diag := range diags {
			if diag.Pos != nil && diag.Pos.Filename != "" {
				diag.Pos.Filename = strings.TrimPrefix(diag.Pos.Filename, folder)
				err = append(err, fmt.Errorf("%s:%d: (%c) %s, %s", diag.Pos.Filename, diag.Pos.Line, diag.Severity, diag.Summary, diag.Detail))
			} else {
				err = append(err, fmt.Errorf("(%c) %s, %s", diag.Severity, diag.Summary, diag.Detail))
			}
		}

		logger.Debugf("Ignored error while trying to load the default variables:\n%v", color.HiBlackString(err.Error()))
	}

	variablesMap := map[string]interface{}{}
	for name, variable := range module.Variables {
		if variable.Default != nil {
			if !template.IsCode(fmt.Sprint(variable.Default)) {
				// The default value contains gotemplate code, we don't want to make it available
				// to gotemplate and we let terraform code initialize the value
				variablesMap[name] = variable.Default
			}
		}
	}
	return variablesMap, module.Variables, nil
}
