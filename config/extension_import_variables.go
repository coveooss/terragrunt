//lint:file-ignore U1000 Ignore all unused code, it's generated

package config

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/multilogger/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

// ImportVariables is a configuration that allows defintion of variables that will be added to
// the current execution. Variables could be defined either by loading files (required or optional)
// or defining vairables directly. It is also possible to define global environment variables.
type ImportVariables struct {
	TerragruntExtensionBase `hcl:",squash"`

	Vars             []string          `hcl:"vars"`
	RequiredVarFiles []string          `hcl:"required_var_files"`
	OptionalVarFiles []string          `hcl:"optional_var_files"`
	Sources          []string          `hcl:"sources"`
	NestedObjects    []string          `hcl:"nested_under"`
	TFVariablesFile  string            `hcl:"output_variables_file"`
	FlattenLevels    *int              `hcl:"flatten_levels"`
	EnvVars          map[string]string `hcl:"env_vars"`
	OnCommands       []string          `hcl:"on_commands"`
}

func (item ImportVariables) itemType() (result string) {
	return ImportVariablesList{}.argName()
}

func (item ImportVariables) help() (result string) {
	if item.Description != "" {
		result += fmt.Sprintf("\n%s\n", item.Description)
	}
	if item.OnCommands != nil {
		result += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(item.OnCommands, ", "))
	}

	return
}

func (item *ImportVariables) normalize() {
	if item.FlattenLevels == nil {
		value := -1
		item.FlattenLevels = &value
	}
	if len(item.NestedObjects) == 0 {
		// By default, we load the variables at the root
		item.NestedObjects = []string{""}
	}
}

func (item *ImportVariables) loadVariablesFromFile(file string, currentVariables map[string]interface{}) (map[string]interface{}, error) {
	item.logger().Debug("Importing ", file)
	vars, err := item.options().LoadVariablesFromFile(file)
	if err != nil {
		return nil, err
	}
	return item.loadVariables(currentVariables, vars, options.VarFile)
}

func (item *ImportVariables) loadVariables(currentVariables map[string]interface{}, newVariables map[string]interface{}, source options.VariableSource) (map[string]interface{}, error) {
	c := item.config()
	for _, nested := range item.NestedObjects {
		c.substitute(&nested)
		imported := newVariables
		if nested != "" {
			imported = map[string]interface{}{nested: imported}
		}
		item.options().ImportVariablesMap(imported, source)
		flattenedVariables := flatten(imported, "", *item.FlattenLevels)
		item.options().ImportVariablesMap(flattenedVariables, source)
		if currentVariables != nil {
			return utils.MergeDictionaries(flattenedVariables, currentVariables)
		}
	}
	return nil, nil
}

func (item *ImportVariables) substituteVars() {
	item.TerragruntExtensionBase.substituteVars()
	c := item.config()
	c.substitute(&item.TFVariablesFile)
	c.substituteEnv(item.EnvVars)
}

// ----------------------- ImportVariablesList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_import_variables.go gen "GenericItem=ImportVariables"
func (list ImportVariablesList) argName() string           { return "import_variables" }
func (list ImportVariablesList) sort() ImportVariablesList { return list }

// Merge elements from an imported list to the current list
func (list *ImportVariablesList) Merge(imported ImportVariablesList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

// ShouldCreateVariablesFile indicates if any import_variables statement requires to
// create a physical variables file
func (list ImportVariablesList) ShouldCreateVariablesFile() bool {
	for _, item := range list.Enabled() {
		if item.TFVariablesFile != "" {
			return true
		}
	}
	return false
}

// Import actually process the variables importers to load and define all variables in the current context
func (list ImportVariablesList) Import() (variablesFiles map[string]map[string]interface{}, err error) {
	variablesFiles = make(map[string]map[string]interface{})

	if len(list) == 0 {
		return variablesFiles, nil
	}

	config := IImportVariables(&list[0]).config()
	terragruntOptions := config.options

	for _, item := range list.Enabled() {
		if len(item.OnCommands) > 0 && !util.ListContainsElement(item.OnCommands, item.options().Env[options.EnvCommand]) {
			// The current command is not in the list of command on which the import should be applied
			return
		}
		item.logger().Debugf("Processing import variables statement %s", item.id())

		if item.TFVariablesFile != "" {
			if _, ok := variablesFiles[item.TFVariablesFile]; !ok {
				variablesFiles[item.TFVariablesFile] = make(map[string]interface{})
			}
		}

		// Set environment variables
		for key, value := range item.EnvVars {
			terragruntOptions.Env[key] = value
		}

		folders := []string{terragruntOptions.WorkingDir}
		var folderErrors errors.Array
		if len(item.Sources) > 0 {
			folders = make([]string, 0, len(item.Sources))
			for _, source := range item.Sources {
				config.substitute(&source)
				if source == "" {
					source = terragruntOptions.WorkingDir
				}
				source, err := config.GetSourceFolder(item.Name, source, true)
				if err != nil {
					folderErrors = append(folderErrors, err)
					continue
				}
				folders = append(folders, source)
			}
		}
		if len(folders) == 0 {
			if len(item.RequiredVarFiles) > 0 {
				return nil, folderErrors
			}
			continue
		}

		// We first process all the -var because they have precedence over -var-file
		keyFunc := func(key string) string { return strings.Split(key, "=")[0] }
		varList := util.RemoveDuplicatesFromList(item.Vars, true, keyFunc)
		for _, varDef := range varList {
			config.substitute(&varDef)
			var (
				key   string
				value interface{}
			)

			if !strings.Contains(varDef, "=") {
				key = varDef
				value = nil
			} else {
				if key, value, err = util.SplitEnvVariable(varDef); err != nil {
					terragruntOptions.Logger.Warningf("-var ignored in %v: %v", item.Name, err)
					continue
				}
			}
			if util.ListContainsElement(terragruntOptions.VariablesExplicitlyProvided(), key) {
				continue
			}

			if variablesFiles[item.TFVariablesFile], err = item.loadVariables(variablesFiles[item.TFVariablesFile], map[string]interface{}{key: value}, options.VarParameter); err != nil {
				return
			}
		}

		// Process RequiredVarFiles
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.RequiredVarFiles) {
			config.substitute(&pattern)

			files := config.globFiles(pattern, true, folders...)
			if len(files) == 0 {
				return nil, fmt.Errorf("%s: No file matches %s", item.name(), pattern)
			}
			for _, file := range files {
				if variablesFiles[item.TFVariablesFile], err = item.loadVariablesFromFile(file, variablesFiles[item.TFVariablesFile]); err != nil {
					return
				}
			}
		}

		// Processes OptionalVarFiles
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.OptionalVarFiles) {
			config.substitute(&pattern)

			for _, file := range config.globFiles(pattern, true, folders...) {
				if util.FileExists(file) {
					if variablesFiles[item.TFVariablesFile], err = item.loadVariablesFromFile(file, variablesFiles[item.TFVariablesFile]); err != nil {
						return
					}
				} else {
					terragruntOptions.Logger.Debugf("Skipping var-file %s as it does not exist", file)
				}
			}
		}
	}

	return
}

func (list ImportVariablesList) Write(variablesFiles map[string]map[string]interface{}, folders ...string) (err error) {
	if len(list) == 0 {
		return nil
	}

	config := IImportVariables(&list[0]).config()
	terragruntOptions := config.options

	for fileName, variables := range variablesFiles {
		if len(variables) == 0 {
			continue
		}
		for _, folder := range folders {
			terragruntOptions.Logger.Debug("Writing terraform variables to directory: " + folder)
			writeTerraformVariables(path.Join(folder, fileName), variables)
		}
	}
	return nil
}

func writeTerraformVariables(fileName string, variables map[string]interface{}) {
	if variables == nil {
		return
	}

	f, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	lines := []string{}

	for key, value := range variables {
		lines = append(lines, fmt.Sprintf("variable \"%s\" {\n", key))
		if value != nil {
			terraformValue := map[string]interface{}{"default": value}
			if _, isMap := value.(map[string]interface{}); isMap {
				terraformValue["type"] = "map"
			} else if _, isList := value.([]interface{}); isList {
				terraformValue["type"] = "list"
			}
			variableContent, err := hcl.MarshalTFVarsIndent(terraformValue, "  ", "  ")
			if err != nil {
				panic(err)
			}
			lines = append(lines, string(variableContent))
		}
		lines = append(lines, "\n}\n\n")
	}
	for _, line := range lines {
		if _, err = f.WriteString(line); err != nil {
			panic(err)
		}
	}
}

func flatten(nestedMap map[string]interface{}, prefix string, numberOfLevels int) map[string]interface{} {
	keysToRemove := []string{}
	itemsToAdd := make(map[string]interface{})

	for key, value := range nestedMap {
		if valueMap, ok := value.(map[string]interface{}); ok {
			isTopLevel := true
			for _, childValue := range valueMap {
				if _, childIsMap := childValue.(map[string]interface{}); childIsMap {
					isTopLevel = false
				}
			}
			if (numberOfLevels < 0 && !isTopLevel) || numberOfLevels >= 1 {
				keysToRemove = append(keysToRemove, key)
				for key, value := range flatten(valueMap, key+"_", numberOfLevels-1) {
					itemsToAdd[key] = value
				}
			}

		}
	}
	newMap := make(map[string]interface{})
	for key, value := range nestedMap {
		newMap[key] = value
	}

	for _, key := range keysToRemove {
		delete(newMap, key)
	}
	for key, value := range itemsToAdd {
		newMap[key] = value
	}
	prefixedMap := make(map[string]interface{})
	for key, value := range newMap {
		prefixedMap[prefix+key] = value
	}
	return prefixedMap
}
