package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
	logging "github.com/op/go-logging"
)

// ExtraCommand is a definition of user extra command that should be executed in place of terraform
type ExtraCommand struct {
	TerragruntExtensionBase `hcl:",squash"`
	Commands                []string
	Aliases                 []string
	Arguments               []string
	ExpandArgs              *bool  `hcl:"expand_args"`
	UseState                *bool  `hcl:"use_state"`
	ActAs                   string `hcl:"act_as"`
	VersionArg              string `hcl:"version"`
}

func (exCmd *ExtraCommand) String() string {
	return fmt.Sprintf("Extra Command %s: %s", exCmd.Name, exCmd.Commands)
}

func (exCmd *ExtraCommand) normalize() {
	if exCmd.Commands == nil {
		// There is no list of commands, so we consider the name to be the exCmd
		exCmd.Commands = []string{exCmd.Name}
	} else if validName.MatchString(exCmd.Name) && !util.ListContainsElement(exCmd.cmdList(), exCmd.Name) {
		// The name is considered as an alias if it match name criteria
		exCmd.Aliases = append(exCmd.Aliases, exCmd.Name)
	}

	def := func(value bool) *bool { return &value }
	if exCmd.UseState == nil {
		exCmd.UseState = def(false)
	}
	if exCmd.ExpandArgs == nil {
		exCmd.ExpandArgs = def(true)
	}
}

var validName = regexp.MustCompile(`^[\w\.-]+$`)

func (exCmd *ExtraCommand) cmdList() []string {
	result := make([]string, 0, len(exCmd.Commands)+len(exCmd.Aliases))
	result = append(result, exCmd.Commands...)
	result = append(result, exCmd.Aliases...)

	for i := range result {
		result[i] = strings.TrimSpace(strings.Split(result[i], "=")[0])
	}
	result = util.RemoveDuplicatesFromListKeepFirst(result)
	sort.Strings(result)
	return result
}

func (exCmd *ExtraCommand) resolve(cmd string) *ActualCommand {
	cmd = exCmd.resolveAlias(cmd)
	if !util.ListContainsElement(exCmd.Commands, cmd) {
		return nil
	}

	var behaveAs string

	if exCmd.ActAs != "" {
		// The command must act as another command for extra argument validation
		exCmd.Options().TerraformCliArgs[0] = exCmd.ActAs
	} else {
		exCmd.ActAs = cmd
		if exCmd.UseState == nil || *exCmd.UseState {
			// We simulate that the extra command acts as the plan command to init the state file
			// and get the modules
			behaveAs = "plan"
		}
	}

	return &ActualCommand{cmd, behaveAs, exCmd}
}

func (exCmd *ExtraCommand) resolveAlias(cmd string) string {
	options := exCmd.Options()

	for _, alias := range exCmd.Aliases {
		split := strings.SplitN(alias, "=", 2)
		if cmd != split[0] {
			continue
		}

		if len(split) == 1 {
			return exCmd.Commands[0]
		}

		cmd = split[1]
		if strings.ContainsAny(split[1], " |,&$") {
			cmd = "bash"

			var args string
			for _, arg := range append(exCmd.Arguments, options.TerraformCliArgs[1:]...) {
				if !strings.Contains(arg, " \t") {
					args += " " + arg
				} else {
					args += fmt.Sprintf(` "%s"`, arg)
				}
			}

			script := split[1]
			if strings.Contains(script, " $*") {
				script = strings.Replace(script, " $*", args, -1)
			} else if !strings.Contains(script, "|") {
				script += args
			}

			exCmd.Arguments = []string{"-c", script}
			options.TerraformCliArgs = options.TerraformCliArgs[:1]
		}
	}
	return cmd
}

// ----------------------- ExtraCommandList -----------------------

// ExtraCommandList represents an array of ExtraCommand objects
type ExtraCommandList []ExtraCommand

// Enabled returns the list of enabled commands
func (cmdList ExtraCommandList) Enabled() ExtraCommandList {
	enabled := make(ExtraCommandList, 0, len(cmdList))
	for _, exCmd := range cmdList {
		if exCmd.Enabled() {
			exCmd.normalize()
			enabled = append(enabled, exCmd)
		}
	}
	return enabled
}

// Help returns the help string for an array of ExtraCommand objects
func (cmdList ExtraCommandList) Help(listOnly bool) string {
	var result string
	sort.Sort(extraCommandsByName(cmdList))

	for _, exCmd := range cmdList.Enabled() {
		result += fmt.Sprintf("\n%s: %s", item(exCmd.Name), strings.Join(exCmd.cmdList(), ", "))
		if listOnly {
			continue
		}
		result += fmt.Sprintln()

		if exCmd.Description != "" {
			result += fmt.Sprintf("\n%s\n", exCmd.Description)
		}

		if exCmd.OS != nil {
			result += fmt.Sprintf("\nApplied only on the following OS: %s\n", strings.Join(exCmd.OS, ", "))
		}

		if exCmd.Arguments != nil {
			result += fmt.Sprintf("\nAutomatically added argument(s): %s\n", strings.Join(exCmd.Arguments, ", "))
		}
	}

	return result
}

// GetVersions returns the the list of versions for extra commands that have a version available
func (cmdList ExtraCommandList) GetVersions() string {
	var result string
	for _, exCmd := range cmdList.Enabled() {
		if exCmd.VersionArg == "" {
			continue
		}
		exCmd.normalize()

		if strings.Contains(exCmd.Name, " ") {
			result += fmt.Sprintf("\n%s\n", item(exCmd.Name))
		}
		for _, cmd := range exCmd.Commands {
			logLevel := logging.GetLevel("")
			if logLevel == logging.NOTICE {
				logging.SetLevel(logging.WARNING, "")
			}
			os.Setenv("TERRAGRUNT_COMMAND", cmd)
			args := []string{exCmd.VersionArg}
			if strings.ContainsAny(exCmd.VersionArg, " |,&$") {
				cmd = "bash"
				args = util.ExpandArguments([]string{"-c", exCmd.VersionArg}, exCmd.config.options.WorkingDir)
			}
			out, err := shell.RunShellCommandAndCaptureOutput(exCmd.config.options, false, cmd, args...)
			logging.SetLevel(logLevel, "")
			if err != nil {
				exCmd.config.options.Logger.Infof("Got %s %s while getting version for %s", color.RedString(err.Error()), out, cmd)
			} else {
				result += fmt.Sprintln(strings.TrimSpace(out))
			}
		}
	}
	return result
}

// ActualCommand returns
func (cmdList ExtraCommandList) ActualCommand(cmd string) ActualCommand {
	for _, exCmd := range cmdList.Enabled() {
		if match := exCmd.resolve(cmd); match != nil {
			return *match
		}
	}
	return ActualCommand{cmd, "", nil}
}

type extraCommandsByName []ExtraCommand

func (e extraCommandsByName) Len() int           { return len(e) }
func (e extraCommandsByName) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e extraCommandsByName) Less(i, j int) bool { return e[i].Name < e[j].Name }

// ActualCommand represents the command that should be executed
type ActualCommand struct {
	Command  string
	BehaveAs string
	Extra    *ExtraCommand
}
