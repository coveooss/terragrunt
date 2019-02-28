package config

import (
	"fmt"
	"github.com/coveo/gotemplate/hcl"
	"github.com/coveo/gotemplate/utils"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

type ImportVariables struct {
	TerragruntExtensionBase `hcl:",squash"`

	Source           string   `hcl:"source"`
	Vars             []string `hcl:"vars"`
	RequiredVarFiles []string `hcl:"required_var_files"`
	OptionalVarFiles []string `hcl:"optional_var_files"`

	NestedUnder string `hcl:"nested_under"`

	TFVariablesFile string `hcl:"output_variables_file"`
	FlattenLevels   *int   `hcl:"flatten_levels"`
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

// ----------------------- ImportVariablesList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_import_variables.go gen "GenericItem=ImportVariables"
func (list ImportVariablesList) argName() string           { return "import_variables" }
func (list ImportVariablesList) sort() ImportVariablesList { return list }

// Merge elements from an imported list to the current list
func (list *ImportVariablesList) Merge(imported ImportVariablesList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

func (list ImportVariablesList) CreatesVariableFile() bool {
	for _, item := range list.Enabled() {
		if item.TFVariablesFile != "" {
			return true
		}
	}
	return false
}

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
		if item.Source != "" {
			newSource, err := config.GetSourceFolder(item.Name, item.Source, len(item.RequiredVarFiles) > 0)
			if err != nil {
				return err
			} else if newSource == "" {
				continue
			}
			folders = []string{newSource}
		}

		// We first process all the -var because they have precedence over -var-file
		// If vars is specified, add -var <key=value> for each specified key
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

			if variablesFiles[item.TFVariablesFile], err = loadVariables(terragruntOptions, &item, variablesFiles[item.TFVariablesFile], map[string]interface{}{key: value}, options.VarParameter); err != nil {
				return err
			}
		}

		// If RequiredVarFiles is specified, add -var-file=<file> for each specified files
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.RequiredVarFiles) {
			pattern = SubstituteVars(pattern, terragruntOptions)

			files := config.globFiles(pattern, folders...)
			if len(files) == 0 {
				return fmt.Errorf("%s: No file matches %s", item.name(), pattern)
			}
			for _, file := range files {
				if variablesFiles[item.TFVariablesFile], err = loadVariablesFromFile(terragruntOptions, &item, file, variablesFiles[item.TFVariablesFile]); err != nil {
					return err
				}
			}
		}

		// If OptionalVarFiles is specified, check for each file if it exists and if so, add -var-file=<file>
		// It is possible that many files resolve to the same path, so we remove duplicates.
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.OptionalVarFiles) {
			pattern = SubstituteVars(pattern, terragruntOptions)

			for _, file := range config.globFiles(pattern, folders...) {
				if util.FileExists(file) {
					if variablesFiles[item.TFVariablesFile], err = loadVariablesFromFile(terragruntOptions, &item, file, variablesFiles[item.TFVariablesFile]); err != nil {
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

func loadVariablesFromFile(terragruntOptions *options.TerragruntOptions, importOptions *ImportVariables, file string, currentVariables map[string]interface{}) (map[string]interface{}, error) {
	terragruntOptions.Logger.Info("Importing", file)
	vars, err := terragruntOptions.LoadVariablesFromFile(file)
	if err != nil {
		return nil, err
	}
	flattenLevels := -1 // flatten all
	if importOptions.FlattenLevels != nil {
		flattenLevels = *importOptions.FlattenLevels
	}
	return loadVariables(terragruntOptions, importOptions, currentVariables, flatten(vars, "", flattenLevels), options.VarFile)
}

func loadVariables(terragruntOptions *options.TerragruntOptions, importOptions *ImportVariables, currentVariables map[string]interface{}, newVariables map[string]interface{}, source options.VariableSource) (map[string]interface{}, error) {
	if importOptions.NestedUnder != "" {
		newVariables = map[string]interface{}{importOptions.NestedUnder: newVariables}
	}
	terragruntOptions.ImportVariablesMap(newVariables, source)
	if currentVariables != nil {
		return utils.MergeDictionaries(newVariables, currentVariables)
	}
	return nil, nil
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
			variableContent, err := hcl.Marshal(terraformValue)
			if err != nil {
				panic(err)
			}
			lines = append(lines, string(variableContent))
		}
		lines = append(lines, "}\n\n")

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
