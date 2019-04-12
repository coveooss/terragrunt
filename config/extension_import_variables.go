package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coveo/gotemplate/v3/errors"
	"github.com/coveo/gotemplate/v3/hcl"
	"github.com/coveo/gotemplate/v3/utils"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

// ImportVariables is a configuration structure to define parameters used to import data files in the terragrunt context
type ImportVariables struct {
	TerragruntExtensionBase `hcl:",squash"`

	Vars              []string    `hcl:"vars"`
	RequiredVarFiles  []string    `hcl:"required_var_files"`
	OptionalVarFiles  []string    `hcl:"optional_var_files"`
	SourcesType       interface{} `hcl:"source"`
	NestedObjectsType interface{} `hcl:"nested_under"`
	TFVariablesFile   string      `hcl:"output_variables_file"`
	FlattenLevels     *int        `hcl:"flatten_levels"`
}

func (item ImportVariables) itemType() (result string) {
	return ImportVariablesList{}.argName()
}

func (item ImportVariables) help() (result string) {
	if item.Description != "" {
		result += fmt.Sprintf("\n%s\n", item.Description)
	}
	return
}

func (item *ImportVariables) stringOrArray(property string, object *interface{}) []string {
	switch source := (*object).(type) {
	case []string:
		for i := range source {
			source[i] = SubstituteVars(source[i], item.options())
		}
		return source
	case string:
		if source != "" {
			return []string{SubstituteVars(source, item.options())}
		}
		return nil
	default:
		item.logger().Warningf("Ignored type (%T) for %s in import_variable %s, type must be string or array of strings", *object, property, item.Name)
		*object = nil
	}
	return nil
}

// Sources always returns an array of source since the user may provide either a string or an array of strings
func (item *ImportVariables) Sources() []string {
	return item.stringOrArray("source", &item.SourcesType)
}

// NestedUnder always returns an array of source since the user may provide either a string or an array of strings
func (item *ImportVariables) NestedUnder() []string {
	return item.stringOrArray("nested_under", &item.NestedObjectsType)
}

func (item *ImportVariables) normalize() {
	if item.FlattenLevels == nil {
		value := -1
		item.FlattenLevels = &value
	}
}

func (item *ImportVariables) loadVariablesFromFile(file string, currentVariables map[string]interface{}) (map[string]interface{}, error) {
	item.logger().Info("Importing", file)
	vars, err := item.options().LoadVariablesFromFile(file)
	if err != nil {
		return nil, err
	}
	return item.loadVariables(currentVariables, vars, options.VarFile)
}

func (item *ImportVariables) loadVariables(currentVariables map[string]interface{}, newVariables map[string]interface{}, source options.VariableSource) (map[string]interface{}, error) {
	for _, nested := range item.NestedUnder() {
		imported := newVariables
		if nested != "" {
			imported = map[string]interface{}{nested: imported}
		}
		item.options().ImportVariablesMap(imported, source)
		if currentVariables != nil {
			return utils.MergeDictionaries(flatten(imported, "", *item.FlattenLevels), currentVariables)
		}
	}
	return nil, nil
}

// ----------------------- ImportVariablesList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_import_variables.go gen "GenericItem=ImportVariables"
func (list ImportVariablesList) argName() string           { return "import_variables" }
func (list ImportVariablesList) sort() ImportVariablesList { return list }

// Merge elements from an imported list to the current list
func (list *ImportVariablesList) Merge(imported ImportVariablesList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

// CreatesVariableFile determines wether a terraform variable file should be created or not
func (list ImportVariablesList) CreatesVariableFile() bool {
	for _, item := range list.Enabled() {
		if item.TFVariablesFile != "" {
			return true
		}
	}
	return false
}

// Import actually process the variables importers to load and define all variables in the current context
func (list ImportVariablesList) Import() (err error) {
	if len(list) == 0 {
		return nil
	}

	config := IImportVariables(&list[0]).config()
	terragruntOptions := config.options

	variablesFiles := make(map[string]map[string]interface{})

	for _, item := range list.Enabled() {
		item.logger().Debugf("Processing import variables statement %s", item.id())

		if item.TFVariablesFile != "" {
			if _, ok := variablesFiles[item.TFVariablesFile]; !ok {
				variablesFiles[item.TFVariablesFile] = make(map[string]interface{})
			}
		}

		folders := []string{terragruntOptions.WorkingDir}
		sources := item.Sources()
		var folderErrors errors.Array
		if sources != nil {
			folders = make([]string, 0, len(sources))
			for i := range sources {
				if sources[i] == "" {
					sources[i] = terragruntOptions.WorkingDir
				}
				newSource, err := config.GetSourceFolder(item.Name, sources[i], true)
				if err != nil {
					folderErrors = append(folderErrors, err)
					continue
				}
				folders = append(folders, newSource)
			}
		}
		if len(folders) == 0 {
			if len(item.RequiredVarFiles) > 0 {
				return folderErrors
			}
			continue
		}

		// We first process all the -var because they have precedence over -var-file
		keyFunc := func(key string) string { return strings.Split(key, "=")[0] }
		varList := util.RemoveDuplicatesFromList(item.Vars, true, keyFunc)
		for _, varDef := range varList {
			varDef = SubstituteVars(varDef, terragruntOptions)
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
				return err
			}
		}

		// Process RequiredVarFiles
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.RequiredVarFiles) {
			pattern = SubstituteVars(pattern, terragruntOptions)

			files := config.globFiles(pattern, true, folders...)
			if len(files) == 0 {
				return fmt.Errorf("%s: No file matches %s", item.name(), pattern)
			}
			for _, file := range files {
				if variablesFiles[item.TFVariablesFile], err = item.loadVariablesFromFile(file, variablesFiles[item.TFVariablesFile]); err != nil {
					return err
				}
			}
		}

		// Processes OptionalVarFiles
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.OptionalVarFiles) {
			pattern = SubstituteVars(pattern, terragruntOptions)

			for _, file := range config.globFiles(pattern, true, folders...) {
				if util.FileExists(file) {
					if variablesFiles[item.TFVariablesFile], err = item.loadVariablesFromFile(file, variablesFiles[item.TFVariablesFile]); err != nil {
						return err
					}
				} else {
					terragruntOptions.Logger.Debugf("Skipping var-file %s as it does not exist", file)
				}
			}
		}
	}

	for fileName, variables := range variablesFiles {
		if len(variables) == 0 {
			continue
		}
		if err := filepath.Walk(terragruntOptions.WorkingDir, func(walkPath string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				return nil
			}
			filesInDir, err := ioutil.ReadDir(walkPath)
			if err != nil {
				return err
			}
			for _, dirFile := range filesInDir {
				if filepath.Ext(dirFile.Name()) == ".tf" {
					terragruntOptions.Logger.Info("Writing terraform variables to directory: " + walkPath)
					writeTerraformVariables(path.Join(walkPath, fileName), variables)
					break
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeTerraformVariables(fileName string, variables map[string]interface{}) {
	if variables == nil {
		return
	}

	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
			variableContent, err := hcl.MarshalIndent(terraformValue, "  ", "  ")
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
	for _, key := range keysToRemove {
		delete(nestedMap, key)
	}
	for key, value := range itemsToAdd {
		nestedMap[key] = value
	}
	newMap := make(map[string]interface{})
	for key, value := range nestedMap {
		newMap[prefix+key] = value
	}
	return newMap
}
