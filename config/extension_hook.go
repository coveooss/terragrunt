package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"

	multiloggerErrors "github.com/coveooss/multilogger/errors"
)

// ----------------------- PreHook implementation -----------------------

// PreHook is a defintion of user command that should be executed before the actual command
type PreHook struct {
	Hook `hcl:",squash"`

	BeforeImports  bool `hcl:"before_imports,optional"`
	AfterInitState bool `hcl:"after_init_state,optional"`
}

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.prehook.go gen TypeName=PreHook
func (list PreHookList) argName() string      { return "pre_hook" }
func (list PreHookList) mergeMode() mergeMode { return mergeModePrepend }

func (list PreHookList) sort() *PreHookList {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Order < list[j].Order })
	return &list
}

// Run execute all pre hooks specified in the list
func (list PreHookList) Run(status error) error { return list.sort().toGeneric().runHooks(status) }

// BeforeImports is a filter function
var BeforeImports = func(hook *PreHook) bool { return hook.BeforeImports }

// BeforeInitState is a filter function
var BeforeInitState = func(hook *PreHook) bool { return !hook.AfterInitState && !hook.BeforeImports }

// AfterInitState is a filter function
var AfterInitState = func(hook *PreHook) bool { return hook.AfterInitState && !hook.BeforeImports }

// ----------------------- PostHook implementation -----------------------

// PostHook is a defintion of user command that should be executed after the actual command
type PostHook struct {
	Hook `hcl:",squash"`
}

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.posthook.go gen TypeName=PostHook
func (list PostHookList) argName() string      { return "post_hook" }
func (list PostHookList) mergeMode() mergeMode { return mergeModeAppend }

func (list PostHookList) sort() *PostHookList {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Order < list[j].Order })
	return &list
}

// Run execute all pre hooks specified in the list
func (list PostHookList) Run(status error) error { return list.sort().toGeneric().runHooks(status) }

// ----------------------- Commont implementation -----------------------

// Hook is a definition of user command that should be executed as part of the terragrunt process
type Hook struct {
	TerragruntExtensionIdentified `hcl:",squash"`

	Command           string            `hcl:"command,optional"`
	Arguments         []string          `hcl:"arguments,optional"`
	ExpandArgs        bool              `hcl:"expand_args,optional"`
	OnCommands        []string          `hcl:"on_commands,optional"`
	IgnoreError       bool              `hcl:"ignore_error,optional"`
	RunOnErrors       bool              `hcl:"run_on_errors,optional"`
	Order             int               `hcl:"order,optional"`
	ShellCommand      bool              `hcl:"shell_command,optional"` // This indicates that the command is a shell command and output should not be redirected
	EnvVars           map[string]string `hcl:"env_vars,optional"`
	PersistentEnvVars map[string]string `hcl:"persistent_env_vars,optional"`
}

func (hook *Hook) id() string  { return hook.Name }
func (hook Hook) values() Hook { return hook }

func (hook Hook) helpDetails() string {
	result := fmt.Sprintf("\nCommand: %s %s\n", hook.Command, strings.Join(hook.Arguments, " "))
	if hook.OnCommands != nil {
		result += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(hook.OnCommands, ", "))
	}
	attributes := []string{
		fmt.Sprintf("Order = %d", hook.Order),
		fmt.Sprintf("Expand arguments = %v", hook.ExpandArgs),
		fmt.Sprintf("Ignore error = %v", hook.IgnoreError),
	}
	result += fmt.Sprintf("\n%s\n", strings.Join(attributes, ", "))
	return result
}

func (hook *Hook) run() error {
	logger := hook.logger()

	if len(hook.OnCommands) > 0 && !util.ListContainsElement(hook.OnCommands, hook.options().Env[options.EnvCommand]) {
		// The current command is not in the list of command on which the hook should be applied
		return nil
	}

	if hook.Command = strings.TrimSpace(hook.Command); hook.Command == "" {
		logger.Debugf("Hook %s skipped, no command to execute", hook.Name)
		return nil
	}

	logger.Infof("Running %s (%s): %s", hook.i.itemType(), hook.id(), hook.name())
	// Add persistent environment variables to the current context (these variables will be available while and after the execution of the hook)
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
	return cmd.Run()
}

func (hook *Hook) setState(err error) {
	exitCode, errCode := shell.GetExitCode(err)
	if errCode != nil {
		exitCode = -1
	}
	hook.options().SetStatus(exitCode, err)
}

type hook interface {
	values() Hook
}

func (list extensionList) runHooks(status error) error {
	if len(list) == 0 {
		return nil
	}

	var errs multiloggerErrors.Array
	var errOccurred bool
	for i := range list {
		hook := list[i].(hook).values()
		if (status != nil || errOccurred) && !hook.RunOnErrors {
			continue
		}
		currentErr := hook.run()
		currentErr = shell.FilterPlanError(currentErr, hook.options().TerraformCliArgs[0])
		if _, ok := currentErr.(tgerrors.PlanWithChanges); ok {
			errs = append(errs, currentErr)
		} else if currentErr != nil && !hook.IgnoreError {
			errOccurred = true
			errs = append(errs, fmt.Errorf("Error while executing %s(%s): %w", hook.itemType(), hook.id(), currentErr))
		}
		hook.setState(currentErr)
	}
	return errs.AsError()
}
