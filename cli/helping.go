package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/fatih/color"
	logging "github.com/op/go-logging"

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

		if strings.Contains(extraCmd.Name, " ") {
			fmt.Printf("\n%s\n", item(extraCmd.Name))
		}
		if extraCmd.Commands == nil {
			if extraCmd.Command == "" {
				extraCmd.Commands = []string{extraCmd.Name}
			} else {
				extraCmd.Commands = []string{extraCmd.Command}
			}
		}
		for _, cmd := range extraCmd.Commands {
			loggingLevel := logging.GetLevel("")
			if loggingLevel == logging.NOTICE {
				logging.SetLevel(logging.WARNING, "")
			}
			os.Setenv("TERRAGRUNT_COMMAND", cmd)
			args := []string{extraCmd.VersionArg}
			if strings.ContainsAny(extraCmd.VersionArg, " |,&$") {
				cmd = "bash"
				args = util.ExpandArguments([]string{"-c", extraCmd.VersionArg}, terragruntOptions.WorkingDir)
			}
			out, err := shell.RunShellCommandAndCaptureOutput(terragruntOptions, false, cmd, args...)
			logging.SetLevel(loggingLevel, "")
			if err != nil {
				terragruntOptions.Logger.Infof("Got %s %s while getting version for %s", color.RedString(err.Error()), out, cmd)
			} else {
				fmt.Println(strings.TrimSpace(out))
			}
		}
	}
}

// PrintDoc prints the contextual documentation relative to the current project
func PrintDoc(terragruntOptions *options.TerragruntOptions, conf *config.TerragruntConfig) {
	doc := printDoc{
		util.ListContainsElement(terragruntOptions.TerraformCliArgs[1:], "list"),
	}

	printTitle := title.Print
	if !doc.listOnly {
		printTitle = title.Println
		fmt.Println(conf.Description)
	}

	printTitle("Extra arguments: (in evaluation order):")
	fmt.Println(util.Indent(doc.extraArgs(conf.Terraform.ExtraArgs), 4))

	printTitle("File importers (in execution order):")
	fmt.Println(util.Indent(doc.importers(conf.ImportFiles, terragruntOptions.WorkingDir), 4))

	printTitle("Pre hooks (in execution order):")
	fmt.Println(util.Indent(doc.hooks(conf.PreHooks, false), 4))
	printTitle("Initialize Terraform state")
	fmt.Println(util.Indent(doc.hooks(conf.PreHooks, true), 4))

	printTitle("Post hooks (in execution order):")
	fmt.Println(util.Indent(doc.hooks(conf.PostHooks), 4))

	printTitle("Extra commands available:")
	fmt.Println(util.Indent(doc.extraCommands(conf.ExtraCommands), 4))
}

type printDoc struct {
	listOnly bool
}

func (pd *printDoc) extraArgs(extraArgs []config.TerraformExtraArguments) (out string) {
	for _, args := range extraArgs {
		out += fmt.Sprintf("\n%s", item(args.Name))
		if pd.listOnly {
			continue
		}
		out += fmt.Sprintln()
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

func (pd *printDoc) importers(importers []config.ImportConfig, cwd string) (out string) {
	for _, importer := range importers {
		out += fmt.Sprintf("\n%s", item(importer.Name))
		if pd.listOnly {
			continue
		}
		out += fmt.Sprintln()
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

func (pd *printDoc) hooks(hooks []config.Hook, afterInitState ...bool) (out string) {
	sort.Sort(hooksByOrder(hooks))

	for _, hook := range hooks {
		if afterInitState != nil && hook.AfterInitState != afterInitState[0] {
			continue
		}
		out += fmt.Sprintf("\n%s", item(hook.Name))
		if pd.listOnly {
			continue
		}
		out += fmt.Sprintln()
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

func (pd *printDoc) extraCommands(extraCommands []config.ExtraCommand) (out string) {
	sort.Sort(extraCommandsByName(extraCommands))
	for _, cmd := range extraCommands {
		var aliases string
		for _, alias := range cmd.Aliases {
			alias = strings.Split(alias, "=")[0]
			if alias == cmd.Name {
				continue
			}
			cmd.Commands = append(cmd.Commands, alias)
		}
		if cmd.Commands != nil {
			cmd.Commands = util.RemoveDuplicatesFromListKeepFirst(cmd.Commands)
			sort.Strings(cmd.Commands)
			aliases = fmt.Sprintf(" | %s", strings.Join(cmd.Commands, " | "))
		}
		out += fmt.Sprintf("\n%s%s", item(cmd.Name), aliases)
		if pd.listOnly {
			continue
		}
		out += fmt.Sprintln()

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
