package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

// TerraformExtraArguments sets a list of arguments to pass to Terraform if command fits any in the `Commands` list
type TerraformExtraArguments struct {
	TerragruntExtensionBase `hcl:",squash"`

	NestedUnder      string   `hcl:"nested_under"`
	Source           string   `hcl:"source"`
	Arguments        []string `hcl:"arguments"`
	Vars             []string `hcl:"vars"`
	RequiredVarFiles []string `hcl:"required_var_files"`
	OptionalVarFiles []string `hcl:"optional_var_files"`
	Commands         []string `hcl:"commands"`
}

func (item TerraformExtraArguments) itemType() (result string) {
	return TerraformExtraArgumentsList{}.argName()
}

func (item TerraformExtraArguments) help() (result string) {
	if item.Description != "" {
		result += fmt.Sprintf("\n%s\n", item.Description)
	}
	if item.Commands != nil {
		result += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(item.Commands, ", "))
	}
	if item.Arguments != nil {
		result += fmt.Sprintf("\nAutomatically add the following parameter(s): %s\n", strings.Join(item.Arguments, ", "))
	}
	return
}

// ----------------------- TerraformExtraArgumentsList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_extra_args.go gen "GenericItem=TerraformExtraArguments"
func (list TerraformExtraArgumentsList) argName() string                   { return "extra_arguments" }
func (list TerraformExtraArgumentsList) sort() TerraformExtraArgumentsList { return list }

// Merge elements from an imported list to the current list
func (list *TerraformExtraArgumentsList) Merge(imported TerraformExtraArgumentsList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

// Filter applies extra_arguments to the current configuration
func (list TerraformExtraArgumentsList) Filter(source string) (result []string, err error) {
	if len(list) == 0 {
		return nil, nil
	}

	config := ITerraformExtraArguments(&list[0]).config()
	terragruntOptions := config.options

	out := []string{}
	cmd := util.IndexOrDefault(terragruntOptions.TerraformCliArgs, 0, "")

	folders := []string{terragruntOptions.WorkingDir}
	if terragruntOptions.WorkingDir != source {
		folders = append(folders, source)
	}

	var arg TerraformExtraArguments

	defer func() {
		if err != nil {
			err = fmt.Errorf("Error while executing %s(%s): %v", arg.itemType(), arg.id(), err)
		}
	}()
	for _, arg = range list.Enabled() {
		arg.logger().Debugf("Processing arg %s", arg.id())
		currentCommandIncluded := util.ListContainsElement(arg.Commands, cmd)

		if currentCommandIncluded {
			out = append(out, arg.Arguments...)
		}

		folders := folders
		logger := arg.logger()

		if arg.Source != "" {
			arg.Source = SubstituteVars(arg.Source, terragruntOptions)
			sourceFolder, err := util.GetSource(arg.Source, filepath.Dir(arg.config().Path), logger)
			if err != nil {
				if len(arg.RequiredVarFiles) > 0 {
					return nil, err
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
			if currentCommandIncluded {
				out = append(out, "-var", varDef)
			}
		}

		// If RequiredVarFiles is specified, add -var-file=<file> for each specified files
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(arg.RequiredVarFiles) {
			files := config.globFiles(pattern, folders...)
			if len(files) == 0 {
				return nil, fmt.Errorf("%s: No file matches %s", arg.name(), pattern)
			}
			for _, file := range files {
				terragruntOptions.Logger.Info("Importing", file)
				if err := terragruntOptions.ImportVariablesFromFile(file, arg.NestedUnder, options.VarFile); err != nil {
					return nil, err
				}
				if currentCommandIncluded {
					out = append(out, fmt.Sprintf("-var-file=%s", file))
				}
			}
		}

		// If OptionalVarFiles is specified, check for each file if it exists and if so, add -var-file=<file>
		// It is possible that many files resolve to the same path, so we remove duplicates.
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(arg.OptionalVarFiles) {
			for _, file := range config.globFiles(pattern, folders...) {
				if util.FileExists(file) {
					if currentCommandIncluded {
						out = append(out, fmt.Sprintf("-var-file=%s", file))
					}
					terragruntOptions.Logger.Info("Importing", file)
					if err := terragruntOptions.ImportVariablesFromFile(file, arg.NestedUnder, options.VarFile); err != nil {
						return nil, err
					}
				} else if currentCommandIncluded {
					terragruntOptions.Logger.Debugf("Skipping var-file %s as it does not exist", file)
				}
			}
		}
	}

	return out, nil
}
