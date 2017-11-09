package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
)

// Hook is a definition of user command that should be executed as part of the terragrunt process
type Hook struct {
	TerragruntExtensionBase `hcl:",squash"`
	Command                 string
	Arguments               []string
	ExpandArgs              bool     `hcl:"expand_args"`
	OnCommands              []string `hcl:"on_commands"`
	IgnoreError             bool     `hcl:"ignore_error"`
	AfterInitState          bool     `hcl:"after_init_state"`
	Order                   int
}

func (hook *Hook) String() string {
	return fmt.Sprintf("Hook %s: %s %s", hook.Name, hook.Command, strings.Join(hook.Arguments, " "))
}

func (hook *Hook) run() error {
	logger := hook.Logger()

	if len(hook.OnCommands) > 0 && !util.ListContainsElement(hook.OnCommands, hook.Options().Env["TERRAGRUNT_COMMAND"]) {
		// The current command is not in the list of command on which the hook should be applied
		return nil
	}

	if !hook.Enabled() {
		logger.Debugf("Hook %s skipped, executed only on %v", hook.Name, hook.OS)
		return nil
	}

	hook.Command = strings.TrimSpace(hook.Command)
	if len(hook.Command) == 0 {
		logger.Debugf("Hook %s skipped, no command to execute", hook.Name)
		return nil
	}

	cmd := shell.RunShellCommand
	if hook.ExpandArgs {
		cmd = shell.RunShellCommandExpandArgs
	}
	if err := cmd(hook.Options(), hook.Command, hook.Arguments...); err != nil && !hook.IgnoreError {
		return fmt.Errorf("%v while running command %s: %s %s", err, hook.Name, hook.Command, strings.Join(hook.Arguments, " "))
	}
	return nil
}

// ----------------------- HookList -----------------------

type hookI interface {
	GetList() *HookList
}
type hookBase struct{}

func (hb hookBase) i() hookI {
	var i interface{} = hb
	return i.(hookI)
}

func (hb hookBase) Help(listOnly bool) string         { return hb.i().GetList().Help(listOnly) }
func (hb hookBase) Run(filters ...HookFilter) error   { return hb.i().GetList().Run(filters...) }
func (hb hookBase) Filter(filter HookFilter) HookList { return hb.i().GetList().Filter(filter) }

type PreHookList struct {
	hookBase
	list HookList `hcl:"pre_hook"`
}

func (hl PreHookList) GetList() *HookList { return &hl.list }

type PostHookList struct {
	hookBase
	list HookList `hcl:"post_hook"`
}

func (hl PostHookList) GetList() *HookList { return &hl.list }

// type PostHookList HookList
type HookList []Hook

// Help returns the help string for an array of Hook objects
func (hooks HookList) Help(listOnly bool) string {
	var result string
	sort.Sort(hooksByOrder(hooks))

	for _, hook := range hooks {
		result += fmt.Sprintf("\n%s", item(hook.Name))
		if listOnly {
			continue
		}
		result += fmt.Sprintln()
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
	}
	return result
}

// Filter returns a list of hook that match the supplied filter
func (hooks HookList) Filter(filter HookFilter) HookList {
	result := make(HookList, 0, len(hooks))
	for _, hook := range hooks {
		if filter(hook) {
			result = append(result, hook)
		}
	}
	return result
}

// Run executes the hooks
func (hooks HookList) Run(filters ...HookFilter) error {
	if len(hooks) == 0 {
		return nil
	}
	sort.Sort(hooksByOrder(hooks))

loop:
	for _, hook := range hooks {
		for _, filter := range filters {
			if !filter(hook) {
				continue loop
			}
		}

		if err := hook.run(); err != nil {
			return err
		}
	}
	return nil
}

// HookFilter is used to filter the hook on supplied criteria
type HookFilter func(Hook) bool

// BeforeInitState is a filter function
var BeforeInitState = func(hook Hook) bool { return !hook.AfterInitState }

// AfterInitState is a filter function
var AfterInitState = func(hook Hook) bool { return hook.AfterInitState }

type hooksByOrder []Hook

func (h hooksByOrder) Len() int           { return len(h) }
func (h hooksByOrder) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h hooksByOrder) Less(i, j int) bool { return h[i].Order < h[j].Order || i < j }
