package config

import (
	"fmt"
	"strings"
)

// TerraformExtraArguments sets a list of arguments to pass to Terraform if command fits any in the `Commands` list
type TerraformExtraArguments struct {
	TerragruntExtensionBase `hcl:",squash"`

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
