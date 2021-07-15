package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/fatih/color"
)

// ExtraCommand is a definition of user extra command that should be executed in place of terraform
type ExtraCommand struct {
	TerragruntExtensionIdentified `hcl:",squash"`

	Commands     []string          `hcl:"commands,optional"`
	Aliases      []string          `hcl:"aliases,optional"`
	Arguments    []string          `hcl:"arguments,optional"`
	ExpandArgs   *bool             `hcl:"expand_args,optional"`
	UseState     *bool             `hcl:"use_state,optional"`
	ActAs        string            `hcl:"act_as,optional"`
	VersionArg   string            `hcl:"version,optional"`
	ShellCommand bool              `hcl:"shell_command,optional"` // This indicates that the command is a shell command and output should not be redirected
	IgnoreError  bool              `hcl:"ignore_error,optional"`
	EnvVars      map[string]string `hcl:"env_vars,optional"`
}

func (item ExtraCommand) extraInfo() string {
	return fmt.Sprintf("[%s]", strings.Join(util.RemoveElementFromList(item.list(), item.Name), ", "))
}

func (item ExtraCommand) helpDetails() string {
	var result string
	if item.Arguments != nil {
		result += fmt.Sprintf("\nAutomatically added argument(s): %s\n", strings.Join(item.Arguments, ", "))
	}
	return result
}

func (item *ExtraCommand) normalize() error {
	if item.Commands == nil {
		// There is no list of commands, so we consider the name to be the extra command
		item.Commands = []string{item.Name}
	} else if validName.MatchString(item.Name) && !util.ListContainsElement(item.list(), item.Name) {
		// The name is considered as an alias if it match name criteria
		item.Aliases = append(item.Aliases, item.Name)
	}

	def := func(value bool) *bool { return &value }
	if item.UseState == nil {
		item.UseState = def(false)
	}
	if item.ExpandArgs == nil {
		item.ExpandArgs = def(true)
	}
	return nil
}

var validName = regexp.MustCompile(`^[\w\.-]+$`)

func (item *ExtraCommand) list() []string {
	result := make([]string, 0, len(item.Commands)+len(item.Aliases))
	result = append(result, item.Commands...)
	result = append(result, item.Aliases...)

	for i := range result {
		result[i] = strings.TrimSpace(strings.Split(result[i], "=")[0])
	}
	result = util.RemoveDuplicatesFromListKeepFirst(result)
	sort.Strings(result)
	return result
}

func (item *ExtraCommand) resolve(cmd string) *ActualCommand {
	cmd, ok := item.resolveAlias(cmd)
	if !util.ListContainsElement(item.Commands, cmd) && !ok {
		return nil
	}

	var behaveAs string

	if item.ActAs != "" {
		// The command must act as another command for extra argument validation
		item.options().TerraformCliArgs[0] = item.ActAs
	} else {
		item.ActAs = cmd
		if item.UseState == nil || *item.UseState {
			// We simulate that the extra command acts as the plan command to init the state file
			// and get the modules
			behaveAs = "plan"
		}
	}

	return &ActualCommand{cmd, behaveAs, item}
}

func (item *ExtraCommand) resolveAlias(cmd string) (result string, found bool) {
	for _, alias := range item.Aliases {
		name, command := collections.Split2(alias, "=")
		if name != cmd {
			continue
		}

		if command == "" {
			return item.Commands[0], true
		}

		return command, true
	}
	return cmd, false
}

// ----------------------- ExtraCommandList -----------------------

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.extra_command.go gen TypeName=ExtraCommand
func (list ExtraCommandList) argName() string      { return "extra_command" }
func (list ExtraCommandList) mergeMode() mergeMode { return mergeModeAppend }

func (list ExtraCommandList) sort() {
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
}

// GetVersions returns the the list of versions for extra commands that have a version available
func (list ExtraCommandList) GetVersions() string {
	var result string
	for _, item := range list.Enabled() {
		if item.VersionArg == "" {
			continue
		}

		if strings.Contains(item.Name, " ") {
			result += fmt.Sprintf("\n%s\n", item.Name)
		}
		for _, cmd := range item.Commands {
			actualCmd := item.VersionArg
			if strings.HasPrefix(actualCmd, "-") {
				// If the command is just a parameter to the actual command, we prefix it with the actual command
				actualCmd = fmt.Sprintf("%s %s", cmd, actualCmd)
			}
			command, tempFile, err := utils.GetCommandFromString(actualCmd)
			if tempFile != "" {
				defer func() { os.Remove(tempFile) }()
				if strings.Contains(actualCmd, "\n") {
					actualCmd = "\n" + actualCmd
				}
			}

			var out string
			if err == nil {
				c := shell.NewCmd(item.options(), command.Args[0])
				c = c.Env(fmt.Sprintf("%s=%s", options.EnvCommand, cmd))
				c = c.Args(append(command.Args[1:], item.options().WorkingDir)...)
				c.DisplayCommand = actualCmd
				out, err = c.Output()
			}
			if err != nil {
				item.logger().Debugf("Got %s %s while getting version for %s", color.RedString(err.Error()), out, item.id())
			} else {
				result += fmt.Sprintln(strings.TrimSpace(out))
			}
		}
	}
	return result
}

// ActualCommand returns
func (list ExtraCommandList) ActualCommand(cmd string) ActualCommand {
	for _, item := range list.Enabled() {
		if match := item.resolve(cmd); match != nil {
			for key, value := range item.EnvVars {
				item.options().Env[key] = value
			}
			return *match
		}
	}
	return ActualCommand{Command: cmd}
}

// ActualCommand represents the command that should be executed
type ActualCommand struct {
	Command  string
	BehaveAs string
	Extra    *ExtraCommand
}
