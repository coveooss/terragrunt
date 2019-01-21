package config

import (
	"fmt"
	"github.com/coveo/gotemplate/utils"
	"os"
	"path"
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

	TFVariablesFile string `hcl:"output_variables_file"`
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

		var variablesFile string
		if item.TFVariablesFile != "" {
			if path.IsAbs(item.TFVariablesFile) {
				variablesFile = item.TFVariablesFile
			} else {
				variablesFile = path.Join(terragruntOptions.WorkingDir, item.TFVariablesFile)
			}
			if _, ok := variablesFiles[variablesFile]; !ok {
				variablesFiles[variablesFile] = make(map[string]interface{})
			}
		}

		folders := []string{terragruntOptions.WorkingDir}

		if newSource, err := config.GetSourceFolder(item.Name, item.Source, len(item.RequiredVarFiles) > 0); err != nil {
			return err
		} else if newSource != "" {
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

			if value != nil {
				terragruntOptions.SetVariable(key, value, options.VarParameter)
			}

			if variablesFile != "" {
				variablesFiles[variablesFile][key] = value
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
				if err := loadVariablesFromFile(terragruntOptions, file, variablesFiles, variablesFile); err != nil {
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
					if err := loadVariablesFromFile(terragruntOptions, file, variablesFiles, variablesFile); err != nil {
						return err
					}
				} else {
					terragruntOptions.Logger.Debugf("Skipping var-file %s as it does not exist", file)
				}
			}
		}

	}

	writeTerraformVariables(variablesFiles)

	return nil
}

func loadVariablesFromFile(terragruntOptions *options.TerragruntOptions, file string, variablesFiles map[string]map[string]interface{}, variablesFileName string) error {
	terragruntOptions.Logger.Info("Importing", file)
	vars, err := terragruntOptions.LoadVariablesFromFile(file)
	if err != nil {
		return err
	}
	terragruntOptions.ImportVariablesMap(vars, options.VarFile)
	if variablesMap, ok := variablesFiles[variablesFileName]; variablesFileName != "" && ok {
		variablesFiles[variablesFileName], err = utils.MergeDictionaries(vars, variablesMap)
	}
	return err
}

func writeTerraformVariables(variablesFiles map[string]map[string]interface{}) {
	for variablesFileName, variablesFile := range variablesFiles {
		f, err := os.OpenFile(variablesFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		defer f.Close()

		lines := []string{}

		for key, value := range flatten(variablesFile, "") {
			lines = append(lines, fmt.Sprintf("variable \"%s\" {\n", key))
			if value != nil {
				lines = append(lines, fmt.Sprintf("  default = \"%v\"\n", value))
			}
			lines = append(lines, "}\n")

		}
		for _, line := range lines {
			if _, err = f.WriteString(line); err != nil {
				panic(err)
			}
		}
	}
}

func flatten(nestedMap map[string]interface{}, prefix string) map[string]interface{} {
	keysToRemove := []string{}
	itemsToAdd := make(map[string]interface{})
	for key, value := range nestedMap {
		if valueMap, ok := value.(map[string]interface{}); ok {
			keysToRemove = append(keysToRemove, key)
			for key, value := range flatten(valueMap, key+"_") {
				itemsToAdd[key] = value
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
