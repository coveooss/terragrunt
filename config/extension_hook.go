//lint:file-ignore U1000 Ignore all unused code, it's generated

package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
)

type HookType int

const (
	UnsetHookType HookType = iota
	PreHookType
	PostHookType
)

func (hookType HookType) String() string {
	switch hookType {
	case UnsetHookType:
		return "(unset hook type!)"
	case PreHookType:
		return "pre_hook"
	case PostHookType:
		return "post_hook"
	default:
		return fmt.Sprintf("(invalid hook type %d!)", hookType)
	}
}

// Hook is a definition of user command that should be executed as part of the terragrunt process
type Hook struct {
	TerragruntExtensionBase `hcl:",remain"`

	Type              HookType
	Command           string            `hcl:"command"`
	Arguments         []string          `hcl:"arguments,optional"`
	ExpandArgs        bool              `hcl:"expand_args,optional"`
	OnCommands        []string          `hcl:"on_commands,optional"`
	IgnoreError       bool              `hcl:"ignore_error,optional"`
	RunOnErrors       bool              `hcl:"run_on_errors,optional"`
	BeforeImports     bool              `hcl:"before_imports,optional"`
	AfterInitState    bool              `hcl:"after_init_state,optional"`
	Order             int               `hcl:"order,optional"`
	ShellCommand      bool              `hcl:"shell_command,optional"` // This indicates that the command is a shell command and output should not be redirected
	EnvVars           map[string]string `hcl:"env_vars,optional"`
	PersistentEnvVars map[string]string `hcl:"persistent_env_vars,optional"`
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

func (hook *Hook) run(args ...interface{}) (result []interface{}, err error) {
	logger := hook.logger()

	if len(hook.OnCommands) > 0 && !util.ListContainsElement(hook.OnCommands, hook.options().Env[options.EnvCommand]) {
		// The current command is not in the list of command on which the hook should be applied
		return
	}

	if !hook.enabled() {
		logger.Debugf("%s %s skipped, executed only on %v", hook.Type, hook.Name, hook.OS)
		return
	}

	logger.Infof("Running %s (%s): %s", hook.Type, hook.id(), hook.name())

	startTime := time.Now()
	defer func() {
		logger.Debugf("Hook timings: %s %s ran in %s", hook.Type, hook.id(), time.Since(startTime))
	}()

	hook.Command = strings.TrimSpace(hook.Command)
	if len(hook.Command) == 0 {
		logger.Debugf("%s %s skipped, no command to execute", hook.Type, hook.Name)
		return
	}

	// Add persistent environment variables to the current context
	// (these variables will be available while and after the execution of the hook)
	for key, value := range hook.PersistentEnvVars {
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

func (hook *Hook) setState(err error) {
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

func (config *TerragruntConfig) initializeHooks() {
	for idx := range config.PreHooks {
		config.PreHooks[idx].Type = PreHookType
	}

	for idx := range config.PostHooks {
		config.PostHooks[idx].Type = PostHookType
	}
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
		if (status != nil || errOccurred) && !hook.RunOnErrors {
			continue
		}
		hook.normalize()
		temp, currentErr := hook.run(args...)
		currentErr = shell.FilterPlanError(currentErr, hook.options().TerraformCliArgs[0])
		if _, ok := currentErr.(tgerrors.PlanWithChanges); ok {
			errs = append(errs, currentErr)
		} else if currentErr != nil && !hook.IgnoreError {
			errOccurred = true
			errs = append(errs, fmt.Errorf("Error while executing %s(%s): %w", hook.itemType(), hook.id(), currentErr))
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
