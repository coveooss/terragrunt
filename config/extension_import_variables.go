package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

type TerraformImportVariables struct {
	TerragruntExtensionBase `hcl:",squash"`

	Source           string   `hcl:"source"`
	Vars             []string `hcl:"vars"`
	RequiredVarFiles []string `hcl:"required_var_files"`
	OptionalVarFiles []string `hcl:"optional_var_files"`
}

func (item TerraformImportVariables) itemType() (result string) {
	return TerraformImportVariablesList{}.argName()
}

func (item TerraformImportVariables) help() (result string) {
	if item.Description != "" {
		result += fmt.Sprintf("\n%s\n", item.Description)
	}
	return
}

// ----------------------- TerraformImportVariablesList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_import_variables.go gen "GenericItem=TerraformImportVariables"
func (list TerraformImportVariablesList) argName() string                    { return "extra_arguments" }
func (list TerraformImportVariablesList) sort() TerraformImportVariablesList { return list }

// Merge elements from an imported list to the current list
func (list *TerraformImportVariablesList) Merge(imported TerraformImportVariablesList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

// Filter applies extra_arguments to the current configuration
func (list TerraformImportVariablesList) Filter(source string) (err error) {
	if len(list) == 0 {
		return nil
	}

	config := ITerraformImportVariables(&list[0]).config()
	terragruntOptions := config.options

	folders := []string{terragruntOptions.WorkingDir}
	if terragruntOptions.WorkingDir != source {
		folders = append(folders, source)
	}

	var arg TerraformImportVariables

	defer func() {
		if err != nil {
			err = fmt.Errorf("Error while executing %s(%s): %v", arg.itemType(), arg.id(), err)
		}
	}()
	for _, arg = range list.Enabled() {
		arg.logger().Debugf("Processing arg %s", arg.id())

		folders := folders
		logger := arg.logger()

		if arg.Source != "" {
			arg.Source = SubstituteVars(arg.Source, terragruntOptions)
			sourceFolder, err := util.GetSource(arg.Source, filepath.Dir(arg.config().Path), logger)
			if err != nil {
				if len(arg.RequiredVarFiles) > 0 {
					return err
				}
				logger.Warningf("%s: %s doesn't exist", arg.Name, arg.Source)
			}
			folders = []string{sourceFolder}
		}

		// We first process all the -var because they have precedence over -var-file
		// If vars is specified, add -var <key=value> for each specified key
		keyFunc := func(key string) string { return strings.Split(key, "=")[0] }
		varList := util.RemoveDuplicatesFromList(arg.Vars, true, keyFunc)
		variablesExplicitlyProvided := terragruntOptions.VariablesExplicitlyProvided()
		for _, varDef := range varList {
			varDef = SubstituteVars(varDef, terragruntOptions)
			if key, value, err := util.SplitEnvVariable(varDef); err != nil {
				terragruntOptions.Logger.Warningf("-var ignored in %v: %v", arg.Name, err)
			} else {
				if util.ListContainsElement(variablesExplicitlyProvided, key) {
					continue
				}
				terragruntOptions.SetVariable(key, value, options.VarParameter)
			}
		}

		// If RequiredVarFiles is specified, add -var-file=<file> for each specified files
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(arg.RequiredVarFiles) {
			files := config.globFiles(pattern, folders...)
			if len(files) == 0 {
				return fmt.Errorf("%s: No file matches %s", arg.name(), pattern)
			}
			for _, file := range files {
				terragruntOptions.Logger.Info("Importing", file)
				if err := terragruntOptions.ImportVariablesFromFile(file, options.VarFile); err != nil {
					return err
				}
			}
		}

		// If OptionalVarFiles is specified, check for each file if it exists and if so, add -var-file=<file>
		// It is possible that many files resolve to the same path, so we remove duplicates.
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(arg.OptionalVarFiles) {
			for _, file := range config.globFiles(pattern, folders...) {
				if util.FileExists(file) {
					terragruntOptions.Logger.Info("Importing", file)
					if err := terragruntOptions.ImportVariablesFromFile(file, options.VarFile); err != nil {
						return err
					}
				} else {
					terragruntOptions.Logger.Debugf("Skipping var-file %s as it does not exist", file)
				}
			}
		}
	}

	return nil
}
