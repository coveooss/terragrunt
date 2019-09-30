package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
)

// Hook is a definition of user command that should be executed as part of the terragrunt process
type Hook struct {
	TerragruntExtensionBase `hcl:",squash"`

	Command        string            `hcl:"command"`
	Arguments      []string          `hcl:"arguments"`
	ExpandArgs     bool              `hcl:"expand_args"`
	OnCommands     []string          `hcl:"on_commands"`
	IgnoreError    bool              `hcl:"ignore_error"`
	BeforeImports  bool              `hcl:"before_imports"`
	AfterInitState bool              `hcl:"after_init_state"`
	Order          int               `hcl:"order"`
	ShellCommand   bool              `hcl:"shell_command"` // This indicates that the command is a shell command and output should not be redirected
	EnvVars        map[string]string `hcl:"env_vars"`
	GlobalVars     map[string]string `hcl:"global_vars"`
}

func (hook Hook) itemType() (result string) { return HookList{}.argName() }

func (hook Hook) help() (result string) {
	if hook.Description != "" {
		result += fmt.Sprintf("\n%s\n", hook.Description)
	}
	result += fmt.Sprintf("\nCommand: %s %s\n", hook.Command, strings.Join(hook.Arguments, " "))
	if hook.OnCommands != nil {
		result += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(hook.OnCommands, ", "))
	}
	if hook.OS != nil {
		result += fmt.Sprintf("\nApplied only on the following OS: %s\n", strings.Join(hook.OS, ", "))
	}
	attributes := []string{
		fmt.Sprintf("Order = %d", hook.Order),
		fmt.Sprintf("Expand arguments = %v", hook.ExpandArgs),
		fmt.Sprintf("Ignore error = %v", hook.IgnoreError),
	}
	result += fmt.Sprintf("\n%s\n", strings.Join(attributes, ", "))
	return
}

func (hook *Hook) substituteVars() {
	hook.TerragruntExtensionBase.substituteVars()
	c := hook.config()
	c.substituteEnv(hook.EnvVars)
	c.substituteEnv(hook.GlobalVars)
	c.substitute(&hook.Command)
	for i, arg := range hook.Arguments {
		hook.Arguments[i] = *c.substitute(&arg)
	}
}

func (hook *Hook) run(args ...interface{}) (result []interface{}, err error) {
	logger := hook.logger()

	if len(hook.OnCommands) > 0 && !util.ListContainsElement(hook.OnCommands, hook.options().Env[options.EnvCommand]) {
		// The current command is not in the list of command on which the hook should be applied
		return
	}

	if !hook.enabled() {
		logger.Debugf("Hook %s skipped, executed only on %v", hook.Name, hook.OS)
		return
	}

	hook.Command = strings.TrimSpace(hook.Command)
	if len(hook.Command) == 0 {
		logger.Debugf("Hook %s skipped, no command to execute", hook.Name)
		return
	}

	// Add global environment variables to the current context
	// (these variables will be available while and after the execution of the hook)
	for key, value := range hook.GlobalVars {
		hook.options().Env[key] = value
	}

	cmd := shell.NewCmd(hook.options(), hook.Command).Args(hook.Arguments...)

	// Add local environment variables to the current context
	// (these variables will be available only while execution of the hook)
	for key, value := range hook.EnvVars {
		cmd.Env(fmt.Sprintf("%s=%s", key, value))
	}
	if hook.ShellCommand {
		// We must not redirect the stderr on shell command, doing so, remove the prompt
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	}

	if hook.ExpandArgs {
		cmd = cmd.ExpandArgs()
	}

	if !utils.IsCommand(hook.Command) {
		cmd.DisplayCommand = fmt.Sprintf("%s %s", hook.name(), strings.Join(hook.Arguments, " "))
	}

	if shouldBeApproved, approvalConfig := hook.config().ApprovalConfig.ShouldBeApproved(hook.Command); shouldBeApproved {
		cmd = cmd.Expect(approvalConfig.ExpectStatements, approvalConfig.CompletedStatements)
	}
	err = cmd.Run()
	return
}

func (hook Hook) setState(err error) {
	exitCode, errCode := shell.GetExitCode(err)
	if errCode != nil {
		exitCode = -1
	}
	hook.options().SetStatus(exitCode, err)
}

// ----------------------- HookList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_hooks.go gen "GenericItem=Hook"
func (list HookList) argName() string { return "hooks" }

func (list HookList) sort() HookList {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Order < list[j].Order })
	return list
}

// MergePrepend prepends elements from an imported list to the current list
func (list *HookList) MergePrepend(imported HookList) {
	list.merge(imported, mergeModePrepend, "pre_hook")
}

// MergeAppend appends elements from an imported list to the current list
func (list *HookList) MergeAppend(imported HookList) {
	list.merge(imported, mergeModeAppend, "post_hook")
}

// Filter returns a list of hook that match the supplied filter
func (list HookList) Filter(filter HookFilter) HookList {
	result := make(HookList, 0, len(list))
	for _, hook := range list.Enabled() {
		if filter(hook) {
			result = append(result, hook)
		}
	}
	return result
}

// HookFilter is used to filter the hook on supplied criteria
type HookFilter func(Hook) bool

// BeforeImports is a filter function
var BeforeImports = func(hook Hook) bool { return hook.BeforeImports }

// BeforeInitState is a filter function
var BeforeInitState = func(hook Hook) bool { return !hook.AfterInitState && !hook.BeforeImports }

// AfterInitState is a filter function
var AfterInitState = func(hook Hook) bool { return hook.AfterInitState && !hook.BeforeImports }

// Run execute the content of the list
func (list HookList) Run(status error, args ...interface{}) (result []interface{}, err error) {
	if len(list) == 0 {
		return
	}

	list.sort()

	var (
		errs        errorArray
		errOccurred bool
	)
	for _, hook := range list {
		if (status != nil || errOccurred) && !hook.IgnoreError {
			continue
		}
		hook.logger().Infof("Running %s (%s): %s", hook.itemType(), hook.id(), hook.name())
		hook.normalize()
		temp, currentErr := hook.run(args...)
		currentErr = shell.FilterPlanError(currentErr, hook.options().TerraformCliArgs[0])
		if currentErr != nil {
			if _, ok := currentErr.(errors.PlanWithChanges); ok {
				errs = append(errs, currentErr)
			} else {
				errOccurred = true
				errs = append(errs, fmt.Errorf("Error while executing %s(%s): %v", hook.itemType(), hook.id(), currentErr))
			}
		}
		hook.setState(currentErr)
		result = append(result, temp)
	}
	switch len(errs) {
	case 0:
		break
	case 1:
		err = errs[0]
	default:
		err = errs
	}
	return
}
