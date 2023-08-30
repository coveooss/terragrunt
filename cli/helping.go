package cli

import (
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/fatih/color"
)

// PrintVersions prints the version of all configured underlying tools
func PrintVersions(terragruntOptions *options.TerragruntOptions, conf *config.TerragruntConfig) {
	fmt.Println("Terragrunt version", terragruntVersion)
	fmt.Println("Terraform version", terraformVersion)
	fmt.Print(conf.ExtraCommands.GetVersions())
}

// PrintDoc prints the contextual documentation relative to the current project
func PrintDoc(terragruntOptions *options.TerragruntOptions, conf *config.TerragruntConfig) {
	var app = kingpin.New("get-doc", "Get documentation about current terragrunt project configuration")
	listOnly := app.Flag("list", "Only list the element names").Short('l').Bool()
	extraArgs := app.Flag("args", "List the extra_arguments configurations").Short('A').Bool()
	imports := app.Flag("imports", "List the import_files configurations").Short('I').Bool()
	variables := app.Flag("variables", "List the import_variables configurations").Short('V').Bool()
	hooks := app.Flag("hooks", "List the pre_hook & post_hook configurations").Short('H').Bool()
	commands := app.Flag("commands", "List the extra_command configurations").Short('C').Bool()
	approvalConfigs := app.Flag("approval-configs", "List the approval configurations").Bool()
	useColor := app.Flag("color", "Enable colors").Short('c').Bool()
	noColor := app.Flag("no-color", "Disable colors").Short('0').Bool()
	filters := app.Arg("filters", "Filter the result").Strings()
	app.HelpFlag.Short('h')
	app.Parse(terragruntOptions.TerraformCliArgs[1:])
	all := !(*hooks || *extraArgs || *imports || *variables || *commands || *approvalConfigs)
	if *noColor {
		color.NoColor = true
	} else if *useColor {
		color.NoColor = false
	}

	title := color.New(color.FgHiWhite)

	print := func(section, format, content string, active bool) {
		if !active && !all || content == "" {
			return
		}
		if section != "" {
			terragruntOptions.Printf("%s:", title.Sprint(section))
			if !*listOnly {
				terragruntOptions.Println()
			}
		}

		terragruntOptions.Printf(format, collections.IndentN(content, 4))
		if *listOnly && content != "" {
			terragruntOptions.Println()
		}
	}

	print("Extra arguments: (in evaluation order)", "%s\n", conf.ExtraArgs.Help(*listOnly, *filters...), *extraArgs)

	if *hooks || all {
		beforeImports := conf.PreHooks.Filter(config.BeforeImports).Help(*listOnly, *filters...)
		print("Pre hooks before imports (in execution order):", "%s\n", beforeImports, true)
	}

	print("Import variables (in execution order)", "%s\n", conf.ImportVariables.Help(*listOnly, *filters...), *variables)
	print("File importers (in execution order)", "%s\n", conf.ImportFiles.Help(*listOnly, *filters...), *imports)
	if *hooks || all {
		pre1 := conf.PreHooks.Filter(config.BeforeInitState).Help(*listOnly, *filters...)
		pre2 := conf.PreHooks.Filter(config.AfterInitState).Help(*listOnly, *filters...)
		post := conf.PostHooks.Help(*listOnly, *filters...)
		print("Pre hooks (in execution order):", "%s", pre1, true)
		if pre1+pre2 != "" {
			terragruntOptions.Println(color.New(color.Faint).Sprint("    Initialize Terraform state"))
		}
		print("", "%s\n", pre2, true)
		if pre1+pre2+post != "" {
			terragruntOptions.Println(color.GreenString("\nRun the actual command\n"))
		}
		print("Post hooks (in execution order)", "%s\n", post, true)
	}
	print("Extra commands available", "%s\n", conf.ExtraCommands.Help(*listOnly, *filters...), *commands)
	print("Approval configurations", "%s\n", conf.ApprovalConfig.Help(*listOnly, *filters...), *approvalConfigs)
}
