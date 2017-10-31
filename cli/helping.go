package cli

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/fatih/color"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
)

var title = color.New(color.FgHiWhite)
var item = color.New(color.FgHiYellow).SprintFunc()

// PrintVersions prints the version of all configured underlying tools
func PrintVersions(terragruntOptions *options.TerragruntOptions, conf *config.TerragruntConfig) {
	fmt.Println("Terragrunt version", terragruntVersion)
	fmt.Println("Terraform version", terraformVersion)
	for _, extraCmd := range conf.ExtraCommands {
		if extraCmd.VersionArg == "" || len(extraCmd.OS) > 0 && !util.ListContainsElement(extraCmd.OS, runtime.GOOS) {
			continue
		}

		fmt.Printf("\n%s\n", item(extraCmd.Name))
		if extraCmd.Commands == nil {
			if extraCmd.Command == "" {
				extraCmd.Commands = []string{extraCmd.Name}
			} else {
				extraCmd.Commands = []string{extraCmd.Command}
			}
		}
		for _, cmd := range extraCmd.Commands {
			title.Printf("\n%s ", cmd)

			if err := shell.RunShellCommand(terragruntOptions, cmd, extraCmd.VersionArg); err != nil {
				terragruntOptions.Logger.Error(err)
			}
		}
	}
}

// PrintDoc prints the contextual documentation relative to the current project
func PrintDoc(terragruntOptions *options.TerragruntOptions, conf *config.TerragruntConfig) {
	fmt.Println(conf.Description)
	title.Println("Extra arguments: (in evaluation order):")
	fmt.Println(util.Indent(extraArgs(conf.Terraform.ExtraArgs), 4))

	title.Println("File importers (in execution order):")
	fmt.Println(util.Indent(importers(conf.ImportFiles, terragruntOptions.WorkingDir), 4))

	title.Println("Pre hooks (in execution order):")
	fmt.Println(util.Indent(hooks(conf.PreHooks, false), 4))
	fmt.Println(util.Indent(item("Initialize Terraform state\n"), 4))
	fmt.Println(util.Indent(hooks(conf.PreHooks, true), 4))

	title.Println("Post hooks (in execution order):")
	fmt.Println(util.Indent(hooks(conf.PostHooks), 4))

	title.Println("Extra commands available:")
	fmt.Println(util.Indent(extraCommands(conf.ExtraCommands), 4))
}

func extraArgs(extraArgs []config.TerraformExtraArguments) (out string) {
	for _, args := range extraArgs {
		out += fmt.Sprintf("\n%s\n", item(args.Name))
		if args.Description != "" {
			out += fmt.Sprintf("\n%s\n", args.Description)
		}
		if args.Commands != nil {
			out += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(args.Commands, ", "))
		}
		if args.Arguments != nil {
			out += fmt.Sprintf("\nAutomatically add the following parameter(s): %s\n", strings.Join(args.Arguments, ", "))
		}
	}
	return
}

func importers(importers []config.ImportConfig, cwd string) (out string) {
	for _, importer := range importers {
		out += fmt.Sprintf("\n%s\n", item(importer.Name))
		if importer.Description != "" {
			out += fmt.Sprintf("\n%s\n", importer.Description)
		}
		if importer.Source != "" {
			out += fmt.Sprintf("\nFrom %s:\n", importer.Source)
		} else {
			out += fmt.Sprint("\nFile(s):\n")
		}

		prefix := importer.Name + "_"
		if importer.Prefix != nil {
			prefix = *importer.Prefix
		}

		target, _ := filepath.Rel(cwd, importer.Target)
		for _, file := range importer.Files {
			target := filepath.Join(target, fmt.Sprintf("%s%s", prefix, filepath.Base(file)))
			if strings.Contains(file, "/terragrunt-cache/") {
				file = filepath.Base(file)
			}
			out += fmt.Sprintf("   %s → %s\n", file, target)
		}

		required := true
		if importer.Required != nil {
			required = *importer.Required
		}

		attributes := []string{fmt.Sprintf("Required = %v", required)}
		if importer.ImportIntoModules {
			attributes = append(attributes, "Import into modules")
		}
		if importer.FileMode != nil {
			attributes = append(attributes, fmt.Sprintf("File mode = %#o", *importer.FileMode))
		}
		out += fmt.Sprintf("\n%s\n", strings.Join(attributes, ", "))
	}
	return
}

func hooks(hooks []config.Hook, afterInitState ...bool) (out string) {
	sort.Sort(hooksByOrder(hooks))

	for _, hook := range hooks {
		if afterInitState != nil && hook.AfterInitState != afterInitState[0] {
			continue
		}
		out += fmt.Sprintf("\n%s\n", item(hook.Name))
		if hook.Description != "" {
			out += fmt.Sprintf("\n%s\n", hook.Description)
		}
		out += fmt.Sprintf("\nCommand: %s %s\n", hook.Command, strings.Join(hook.Arguments, " "))
		if hook.OnCommands != nil {
			out += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(hook.OnCommands, ", "))
		}
		if hook.OS != nil {
			out += fmt.Sprintf("\nApplied only on the following OS: %s\n", strings.Join(hook.OS, ", "))
		}
		attributes := []string{
			fmt.Sprintf("Order = %d", hook.Order),
			fmt.Sprintf("Expand arguments = %v", hook.ExpandArgs),
			fmt.Sprintf("Ignore error = %v", hook.IgnoreError),
		}
		out += fmt.Sprintf("\n%s\n", strings.Join(attributes, ", "))
	}
	return
}

type extraCommandsByName []config.ExtraCommand

func (e extraCommandsByName) Len() int      { return len(e) }
func (e extraCommandsByName) Swap(i, j int) { e[i], e[j] = e[j], e[i] }
func (e extraCommandsByName) Less(i, j int) bool {
	return e[i].Name < e[j].Name
}

func extraCommands(extraCommands []config.ExtraCommand) (out string) {
	sort.Sort(extraCommandsByName(extraCommands))
	for _, cmd := range extraCommands {
		var aliases string
		commands := append(cmd.Commands, cmd.Aliases...)
		if commands != nil {
			sort.Strings(commands)
			aliases = fmt.Sprintf(" | %s", strings.Join(commands, " | "))
		}
		out += fmt.Sprintf("\n%s%s\n", item(cmd.Name), aliases)

		if cmd.Description != "" {
			out += fmt.Sprintf("\n%s\n", cmd.Description)
		}

		if cmd.OS != nil {
			out += fmt.Sprintf("\nApplied only on the following OS: %s\n", strings.Join(cmd.OS, ", "))
		}

		if cmd.Arguments != nil {
			out += fmt.Sprintf("\nAutomatically added argument(s): %s\n", strings.Join(cmd.Arguments, ", "))
		}
	}
	return
}
