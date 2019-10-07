package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/coveooss/gotemplate/v3/json"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/gotemplate/v3/yaml"
)

// LoadDefaultValues returns a map of the variables defined in the tfvars file
func LoadDefaultValues(folder string) (result map[string]interface{}, err error) {
	result = map[string]interface{}{}
	for _, file := range getTerraformFiles(folder) {
		var fileVars map[string]interface{}
		switch filepath.Ext(file) {
		case ".tf":
			fileVars, err = getDefaultVarsFromFile(file, hcl.Unmarshal)
		case ".json":
			fileVars, err = getDefaultVarsFromFile(file, json.Unmarshal)
		}
		if err != nil {
			return nil, err
		}

		for key, value := range fileVars {
			if old, exist := result[key]; exist {
				switch old := old.(type) {
				case map[string]interface{}:
					newValue, ok := value.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("Cannot override %s: %T with %T", key, old, value)
					}
					value, err = utils.MergeDictionaries(old, newValue)
					if err != nil {
						return nil, err
					}
				}
			}
			result[key] = value
		}
	}

	return
}

// LoadVariablesFromFile returns a map of the variables defined in the tfvars file
func LoadVariablesFromFile(path, cwd string, context ...interface{}) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if result, err := LoadVariablesFromSource(string(bytes), path, cwd, context...); err != nil {
		return nil, err
	} else {
		if len(result) == 1 && result["variable"] != nil {
			return getDefaultVars(result)
		}
		return result, nil
	}
}

// LoadVariables returns a map of the variables defined in the content provider
func LoadVariables(content string, cwd string, context ...interface{}) (map[string]interface{}, error) {
	return LoadVariablesFromSource(content, "Terragrunt content", cwd, context...)
}

// LoadVariablesFromSource returns a map of the variables defined in the content provider
func LoadVariablesFromSource(content, fileName, cwd string, context ...interface{}) (result map[string]interface{}, err error) {
	result = make(map[string]interface{})
	if ApplyTemplate() && template.IsCode(content) {
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
			template.SetLogLevel(GetLoggingLevel())
			if modifiedContent, err := t.ProcessContent(content, fileName); err != nil {
				// In case of error, we simply issue a warning and continue with the original content
				template.Log.Warning(err)
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
			err = fmt.Errorf("Error in %s:%s", fileName, strings.TrimPrefix(err.Error(), "At "))
		}
	}
	return
}

// ApplyTemplate determines if go template should be applied on terraform files.
func ApplyTemplate() bool {
	template := os.Getenv("TERRAGRUNT_TEMPLATE")
	switch strings.ToLower(template) {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

// Returns the list of terraform files in a folder in alphabetical order (override files are always at the end)
func getTerraformFiles(folder string) []string {
	matches := map[string]int{}

	// Resolve all patterns and add them to the matches map. Since the order is important (i.e. override files comes after non
	// overridden files, we store the last pattern index in the map). f_override.tf will match both *.tf and *_override.tf, but
	// will be associated with *_override.tf at the end, which is what is expected.
	for i, pattern := range patterns {
		files, err := filepath.Glob(filepath.Join(folder, pattern))
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			matches[file] = i
		}
	}

	// Then, we group files in two categories (regular and override) and we sort them alphabetically
	var regularsFiles, overrideFiles []string
	for file, index := range matches {
		list := &regularsFiles
		if index >= 2 {
			// We group overrides files together
			list = &overrideFiles
		}
		*list = append(*list, file)
	}
	sort.Strings(regularsFiles)
	sort.Strings(overrideFiles)
	return append(regularsFiles, overrideFiles...)
}

var patterns = []string{"*.tf", "*.tf.json", "override.tf", "override.tf.json", "*_override.tf", "*_override.tf.json"}

func getDefaultVarsFromFile(filename string, unmarshal func([]byte, interface{}) error) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	content := make(map[string]interface{})
	if err := unmarshal(bytes, &content); err != nil {
		_, filename = filepath.Split(filename)
		return nil, fmt.Errorf("%v %v", filename, err)
	}

	result, err := getDefaultVars(content)
	if err != nil {
		return result, fmt.Errorf("%v: %v", filename, err)
	}

	return result, nil
}

func getDefaultVars(content map[string]interface{}) (map[string]interface{}, error) {
	var err error
	result := make(map[string]interface{})

	switch variables := content["variable"].(type) {
	case map[string]interface{}:
		addVariables(variables, result)
	case []interface{}:
		for _, value := range variables {
			addVariables(value.(map[string]interface{}), result)
		}
	case nil:
	default:
		return nil, fmt.Errorf("Unknown variable type %[1]T: %[1]v", variables)
	}

	switch locals := content["locals"].(type) {
	case map[string]interface{}:
		result["local"] = locals
	case []interface{}:
		localMaps := make([]map[string]interface{}, len(locals))
		for i := range localMaps {
			localMaps[i] = locals[i].(map[string]interface{})
		}
		result["local"], err = utils.MergeDictionaries(localMaps...)
	case nil:
	default:
		return nil, fmt.Errorf("Unknown local type %T", locals)
	}

	return result, err
}

func addVariables(source, target map[string]interface{}) {
	for name, value := range source {
		value := value.(map[string]interface{})
		if value := value["default"]; value != nil {
			target[name] = value
		}
	}
}
