package config

import (
	"fmt"
	"strings"

	"github.com/coveooss/terragrunt/v2/util"
)

// TerraformExtraArguments sets a list of arguments to pass to Terraform if command fits any in the `Commands` list
type TerraformExtraArguments struct {
	TerragruntExtensionBase `hcl:",remain"`

	Source           string            `hcl:"source,optional"`
	Arguments        []string          `hcl:"arguments,optional"`
	RequiredVarFiles []string          `hcl:"required_var_files,optional"`
	OptionalVarFiles []string          `hcl:"optional_var_files,optional"`
	Commands         []string          `hcl:"commands,optional"`
	EnvVars          map[string]string `hcl:"env_vars,optional"`
}

func (item TerraformExtraArguments) itemType() (result string) {
	return TerraformExtraArgumentsList{}.argName()
}

func (item TerraformExtraArguments) onCommand() []string { return item.Commands }

func (item TerraformExtraArguments) helpDetails() string {
	var result string
	if item.Arguments != nil {
		result += fmt.Sprintf("\nAutomatically add the following parameter(s): %s\n", strings.Join(item.Arguments, ", "))
	}
	return result
}

// ----------------------- TerraformExtraArgumentsList -----------------------

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.extra_args.go gen Type=TerraformExtraArguments
func (list TerraformExtraArgumentsList) argName() string { return "extra_arguments" }

// Merge elements from an imported list to the current list
func (list *TerraformExtraArgumentsList) Merge(imported TerraformExtraArgumentsList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

// Filter applies extra_arguments to the current configuration
func (list TerraformExtraArgumentsList) Filter(source string) (result []string, err error) {
	if len(list) == 0 {
		return nil, nil
	}

	config := list[0].config()
	terragruntOptions := config.options

	out := []string{}
	cmd := util.IndexOrDefault(terragruntOptions.TerraformCliArgs, 0, "")

	for _, arg := range list.Enabled() {
		arg.logger().Debugf("Processing arg %s", arg.id())

		if !util.ListContainsElement(arg.Commands, cmd) {
			continue
		}

		// Append arguments
		out = append(out, arg.Arguments...)

		folders := []string{terragruntOptions.WorkingDir}
		if terragruntOptions.WorkingDir != source {
			folders = append(folders, source)
		}

		if newSource, err := config.GetSourceFolder(arg.Name, arg.Source, len(arg.RequiredVarFiles) > 0, ""); err != nil {
			return nil, err
		} else if newSource != "" {
			folders = []string{newSource}
		}

		// Set the environment variables
		for key, value := range arg.EnvVars {
			terragruntOptions.Env[key] = value
		}

		// If RequiredVarFiles is specified, add -var-file=<file> for each specified files
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(arg.RequiredVarFiles) {
			files := config.globFiles(pattern, false, folders...)
			if len(files) == 0 {
				return nil, fmt.Errorf("%s: No file matches %s", arg.name(), pattern)
			}
			for _, file := range files {
				out = append(out, fmt.Sprintf("-var-file=%s", file))
			}
		}

		// If OptionalVarFiles is specified, check for each file if it exists and if so, add -var-file=<file>
		// It is possible that many files resolve to the same path, so we remove duplicates.
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(arg.OptionalVarFiles) {
			for _, file := range config.globFiles(pattern, false, folders...) {
				if util.FileExists(file) {
					out = append(out, fmt.Sprintf("-var-file=%s", file))
				} else {
					terragruntOptions.Logger.Tracef("Skipping var-file %s as it does not exist", file)
				}
			}
		}
	}

	return out, nil
}
