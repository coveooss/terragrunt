package util

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform/configs"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/json"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/gotemplate/v3/yaml"
	"github.com/coveooss/multilogger"
	"github.com/coveooss/multilogger/errors"
	"github.com/sirupsen/logrus"
)

type (
	cachedDefaultVariables struct {
		values map[string]interface{}
		all    map[string]*configs.Variable
		err    error
	}
)

var (
	reTfVariables = regexp.MustCompile(`(?ms)^variable\s+".*?"\s+{(\s*\n.*?^}|\s*}$)`)
	cacheDefault  sync.Map
)

// LoadDefaultValues returns a map of the variables defined in the tfvars file
func LoadDefaultValues(folder string, logger *multilogger.Logger, keepInCache bool) (importedVariables map[string]interface{}, allVariables map[string]*configs.Variable, err error) {
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

	return loadDefaultValues(folder, true, logger)
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
			return getTerraformVariableValues(terraformConfig)
		}
	}

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

func getTerraformVariableValues(terraformConfig interface{}) (map[string]interface{}, error) {
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
		if !variable.Default.IsNull() {
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

func loadDefaultValues(folder string, retry bool, logger *multilogger.Logger) (map[string]interface{}, map[string]*configs.Variable, error) {
	terraformConfig, diag := configs.NewParser(nil).LoadConfigDir(folder)
	if diag.HasErrors() {
		if err := convertHclError(diag); err != nil {
			err = fmt.Errorf("caught errors while trying to load default variable values from %s:\n%v", folder, err)
			if !retry {
				return nil, nil, err
			}
			// If we get an error while loading variables, we try to isolate variables definition in a temporay folder.
			if logger != nil {
				logger.Debugf("First try with terraform native LoadConfigDir failed\n%v", color.HiBlackString(err.Error()))
				logger.Debug("Retrying with a temporary folder containing only variables deinitions")
			}
			tmpDir, err := createTerraformVariablesTemporaryFolder(folder)
			if tmpDir == "" || err != nil {
				return nil, nil, err
			}
			defer func() { os.RemoveAll(tmpDir) }()
			return loadDefaultValues(tmpDir, false, logger)
		}
	}
	importedVariables, err := getTerraformVariableValues(terraformConfig)
	return importedVariables, terraformConfig.Variables, err
}

func isOverride(filename string) bool {
	return path.Base(filename) == "override.tf" ||
		path.Base(filename) == "override.tf.json" ||
		strings.HasSuffix(filename, "_override.tf") ||
		strings.HasSuffix(filename, "_override.tf.json")
}

func convertHclError(err hcl.Diagnostics) error {
	if !err.HasErrors() {
		return nil
	}
	var x errors.Array
	return append(x, err.Errs()...)
}

// Create a temporary folder containing only the terraform code used to declare and initialize variables.
// This allows us to load the default variable value to make them available to gotemplate code before we
// apply gotemplate on terraform file. If we don't, any terraform code that is not considered as valid
// code will cause an error and make LoadDefaultValues fail.
func createTerraformVariablesTemporaryFolder(folder string) (tmpDir string, err error) {
	var files []string

	if _, err = os.Stat(folder); err != nil {
		return
	}
	if files, err = utils.FindFiles(folder, false, false, "*.tf", "*.tf.json"); err != nil {
		return
	}

	// The variables are splitted up into 4 different files depending if they are declared
	// in a .tf file or a .tf.json file and if they are declareds in an override file or not.
	generated := make(map[string]interface{})
	for _, filename := range files {
		name, ext := "regular", path.Ext(filename)
		if isOverride(filename) {
			name = "override"
		}

		var content []byte
		if content, err = os.ReadFile(filename); err != nil {
			return
		}

		if ext == ".tf" {
			if found := reTfVariables.FindAllString(string(content), -1); len(found) > 0 {
				currentValue, _ := generated[name+ext].(string)
				generated[name+ext] = currentValue + fmt.Sprintf("// %s\n%s\n", filename, strings.Join(found, "\n\n"))
			}
		} else {
			ext = ".tf.json"
			var jsonContent json.Dictionary
			if err = json.Unmarshal(content, &jsonContent); err == nil {
				variables := jsonContent.Clone("variable")
				if variables.Len() > 0 {
					currentValue, _ := generated[name+ext].(json.Dictionary)
					generated[name+ext] = variables.Merge(currentValue)
				}
			}
		}
	}

	if len(generated) > 0 {
		// We write the resulting files
		if tmpDir, err = os.MkdirTemp("", "load_defaults"); err != nil {
			return
		}
		for filename, value := range generated {
			var content []byte
			switch value := value.(type) {
			case string:
				content = []byte(value)
			case json.Dictionary:
				content = []byte(value.PrettyPrint())
			}
			if err = os.WriteFile(path.Join(tmpDir, filename), content, 0644); err != nil {
				return
			}
		}
	}
	return
}
