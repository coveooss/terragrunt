package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

// PrintVersions prints the version of all configured underlying tools
func PrintVersions(terragruntOptions *options.TerragruntOptions, conf *config.TerragruntConfig) {
	fmt.Println("Terragrunt version", terragruntVersion)
	fmt.Println("Terraform version", terraformVersion)
	fmt.Print(conf.ExtraCommands.GetVersions())
}

// PrintDoc prints the contextual documentation relative to the current project
func PrintDoc(terragruntOptions *options.TerragruntOptions, conf *config.TerragruntConfig) {
	listOnly := util.ListContainsElement(terragruntOptions.TerraformCliArgs[1:], "list")
	title := color.New(color.FgHiWhite)

	printTitle := title.Print
	if !listOnly {
		printTitle = title.Println
		fmt.Println(conf.Description)
	}

	printTitle("Extra arguments: (in evaluation order):")
	fmt.Println(util.Indent(conf.Terraform.ExtraArgs.Help(listOnly), 4))

	printTitle("File importers (in execution order):")
	fmt.Println(util.Indent(conf.ImportFiles.Help(listOnly), 4))

	printTitle("Pre hooks (in execution order):")
	hooks := conf.PreHooks.Filter(config.BeforeInitState)
	fmt.Println(util.Indent(hooks.Help(listOnly), 4))
	printTitle("Initialize Terraform state")
	hooks = conf.PreHooks.Filter(config.AfterInitState)
	fmt.Println(util.Indent(hooks.Help(listOnly), 4))

	printTitle("Post hooks (in execution order):")
	fmt.Println(util.Indent(conf.PostHooks.Help(listOnly), 4))

	printTitle("Extra commands available:")
	fmt.Println(util.Indent(conf.ExtraCommands.Help(listOnly), 4))
}
