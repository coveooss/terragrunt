package config

import (
	"fmt"
	"strings"
)

// TerraformExtraArguments sets a list of arguments to pass to Terraform if command fits any in the `Commands` list
type TerraformExtraArguments struct {
	TerragruntExtensionBase `hcl:",squash"`
	Arguments               []string
	Vars                    []string
	RequiredVarFiles        []string `hcl:"required_var_files"`
	OptionalVarFiles        []string `hcl:"optional_var_files"`
	Commands                []string
}

func (conf *TerraformExtraArguments) String() string {
	return fmt.Sprintf("TerraformArguments{Name = %s, Arguments = %v, Commands = %v}", conf.Name, conf.Arguments, conf.Commands)
}

// ----------------------- TerraformExtraArgumentsList -----------------------

// TerraformExtraArgumentsList represents an array of TerraformExtraArguments objects
type TerraformExtraArgumentsList []TerraformExtraArguments

// Help returns the help string for an array of Hook objects
func (eal TerraformExtraArgumentsList) Help(listOnly bool) string {
	var result string

	for _, args := range eal {
		result += fmt.Sprintf("\n%s", item(args.Name))
		if listOnly {
			continue
		}
		result += fmt.Sprintln()
		if args.Description != "" {
			result += fmt.Sprintf("\n%s\n", args.Description)
		}
		if args.Commands != nil {
			result += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(args.Commands, ", "))
		}
		if args.Arguments != nil {
			result += fmt.Sprintf("\nAutomatically add the following parameter(s): %s\n", strings.Join(args.Arguments, ", "))
		}
	}
	return result
}
