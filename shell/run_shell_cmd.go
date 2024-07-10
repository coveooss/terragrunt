package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/multilogger"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/sirupsen/logrus"
)

// CommandContext is the description of the command that must be executed
type CommandContext struct {
	Stdout, Stderr io.Writer // If these variables are left unset, the default value from current options set will be used
	DisplayCommand string    // If not specified, the actual command will be displayed
	LogLevel       logrus.Level

	command             string
	options             *options.TerragruntOptions
	args                []interface{}
	expandArgs          bool
	expectedStatements  []string
	completedStatements []string
	log                 *multilogger.Logger
	env                 []string
	workingDir          string
	retries             int
}

// NewCmd initializes the ShellCommand object
func NewCmd(options *options.TerragruntOptions, cmd string) *CommandContext {
	context := CommandContext{
		command:    cmd,
		options:    options,
		log:        options.Logger,
		env:        options.EnvironmentVariables(),
		workingDir: options.WorkingDir,
		LogLevel:   logrus.DebugLevel,
	}
	return &context
}

// NewTFCmd initializes the ShellCommand object with terraform as the command
func NewTFCmd(options *options.TerragruntOptions) *CommandContext {
	return NewCmd(options, options.TerraformPath)
}

// Args add arguments to the CommandContext
func (c *CommandContext) Args(args ...string) *CommandContext {
	for i := range args {
		c.args = append(c.args, args[i])
	}
	return c
}

// ExpandArgs instructs that arguments should be expanded before run
func (c *CommandContext) ExpandArgs() *CommandContext {
	c.expandArgs = true
	return c
}

// Expect instructs that a special behavior should be done on some outputs
func (c *CommandContext) Expect(expected []string, completed []string) *CommandContext {
	c.expectedStatements = expected
	c.completedStatements = completed
	return c
}

// Env set additional environment variables in the CommandContext
func (c *CommandContext) Env(values ...string) *CommandContext {
	c.env = append(c.env, values...)
	return c
}

// WithRetries adds a number retries to commands before failing
func (c *CommandContext) WithRetries(retries int) *CommandContext {
	c.retries = retries
	return c
}

// WorkingDir changes the default working directory for the command
func (c *CommandContext) WorkingDir(wd string) *CommandContext {
	c.workingDir = wd
	return c
}

// Output runs the current command and returns the output (stdout and stderr)
func (c CommandContext) Output() (string, error) {
	out := new(bytes.Buffer)
	c.Stdout, c.Stderr = out, out
	err := c.Run()
	return out.String(), err
}

// LogOutput runs the current command and log the output (stdout and stderr)
func (c CommandContext) LogOutput(logLevel logrus.Level) error {
	out, err := c.Output()
	if err != nil {
		c.log.Error(out)
	} else {
		c.log.Log(logLevel, out)
	}
	return err
}

// Run executes the command
func (c CommandContext) Run() error {
	if c.options == nil {
		return tgerrors.WithStackTrace(fmt.Errorf("options not configured for command"))
	}

	// If the output is captured, we use a different logging level
	c.Stdout = iif(c.Stdout, c.Stdout, c.options.Writer).(io.Writer)
	c.Stderr = iif(c.Stderr, c.Stderr, c.options.ErrWriter).(io.Writer)

	if c.command == c.options.TerraformPath {
		const noColor = "-no-color"
		if util.ListContainsElement(c.options.TerraformCliArgs, noColor) {
			// If the user specified -no-color, we should respect it in intermediate calls too
			c.args = append(c.args, noColor)
		}
		// Terragrunt can run some commands (such as terraform remote config) before running the actual terraform
		// command requested by the user. The output of these other commands should not end up on stdout as this
		// breaks scripts relying on terraform's output.
		if !reflect.DeepEqual(collections.ToInterfaces(c.options.TerraformCliArgs...), c.args) {
			c.Stdout = c.Stderr
		}
	}

	if c.expandArgs {
		c.args = util.ExpandArguments(c.args, c.options.WorkingDir)
	}

	if utils.IsCommand(c.command) {
		// We try to resolve the command with the options PATH since it is not necessary equal to the actual PATH
		// and therefore, the resolution of the command name may be altered
		if resolvedCommand, err := LookPath(c.command, c.options.Env["PATH"]); err == nil && resolvedCommand != c.command {
			c.command = resolvedCommand
		}
	}

	var finalStatus error
	for try := 0; try <= c.retries; try++ {
		cmd, tempFile, err := utils.GetCommandFromString(c.command, c.args...)
		if err != nil {
			return tgerrors.WithStackTrace(err)
		}

		if cmd.Args[0], err = LookPath(cmd.Args[0], c.options.Env["PATH"]); err != nil {
			return tgerrors.WithStackTrace(err)
		}

		verb := "Running "
		if try > 0 {
			verb = fmt.Sprintf("Trying(#%d)", try+1)
			// On subsequent retry, we ignore the output to avoid displaying the same output many times
			// TODO, check if the output is the same as the previous one to catch different messages
			c.Stdout, c.Stderr = nil, nil
		}

		if c.DisplayCommand == "" {
			c.DisplayCommand = filepath.Base(cmd.Args[0])
		}
		c.log.Log(c.LogLevel, verb, "command: ", c.DisplayCommand, " ", strings.Join(cmd.Args[1:], " "))

		if tempFile != "" {
			content, _ := os.ReadFile(tempFile)
			if c.DisplayCommand == "" {
				c.options.Logger.Debugf("\n%s", string(content))
			}
			defer func() { os.Remove(tempFile) }()
		}

		cmd.Stdout, cmd.Stderr, cmd.Env = c.Stdout, c.Stderr, c.env
		cmd.Dir = c.options.WorkingDir
		cmdChannel := make(chan error)

		signalChannel := NewSignalsForwarder(forwardSignals, cmd, c.log, cmdChannel)
		defer signalChannel.Close()

		if c.expectedStatements != nil && c.completedStatements != nil {
			finalStatus = RunCommandToApprove(cmd, c.expectedStatements, c.completedStatements, c.options)
		} else {
			cmd.Stdin = os.Stdin
			finalStatus = cmd.Run()
		}

		cmdChannel <- finalStatus
		if finalStatus == nil {
			break
		} else {
			c.log.Debugf("Caught error on command: %v", finalStatus)
		}
	}

	return tgerrors.WithStackTrace(finalStatus)
}

// LookPath search the supplied path to find the desired command
// It uses a mutex since it has to temporary override the global PATH variable.
func LookPath(command string, paths ...string) (string, error) {
	originalPath := os.Getenv("PATH")
	testPath := strings.Join(paths, string(os.PathListSeparator))

	if testPath != "" {
		defer func() {
			os.Setenv("PATH", originalPath)
			lookPathMutex.Unlock()
		}()

		lookPathMutex.Lock()
		os.Setenv("PATH", testPath)
	}
	return exec.LookPath(command)
}

var lookPathMutex sync.Mutex

// GetExitCode returns the exit code of a command. If the error does not implement tgerrors.IErrorCode or is not an exec.ExitError type,
// the error is returned.
func GetExitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := tgerrors.Unwrap(err).(tgerrors.IErrorCode); ok {
		return exitErr.ExitStatus()
	}

	if exitErr, ok := tgerrors.Unwrap(err).(*exec.ExitError); ok {
		status := exitErr.Sys().(syscall.WaitStatus)
		return status.ExitStatus(), nil
	}
	return 0, err
}

// FilterPlanError filters to trap plan exit code that are not real error
func FilterPlanError(err error, command string) error {
	if exitCode, err := GetExitCode(err); err == nil && command == "plan" && exitCode == tgerrors.ChangeExitCode {
		// For plan, an error with exit code 2 should not be considered as a real error
		return tgerrors.PlanWithChanges{}
	}
	return err
}

// SignalsForwarder forwards signals to a command, waiting for the command to finish.
type SignalsForwarder chan os.Signal

// NewSignalsForwarder returns a new SignalsForwarder
func NewSignalsForwarder(signals []os.Signal, c *exec.Cmd, logger *multilogger.Logger, cmdChannel chan error) SignalsForwarder {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, signals...)

	go func() {
		for {
			select {
			case s := <-signalChannel:
				logger.Warningf("Forward signal %v to terraform.", s)
				err := c.Process.Signal(s)
				if err != nil {
					logger.Errorf("Error forwarding signal: %v", err)
				}
			case <-cmdChannel:
				return
			}
		}
	}()

	return signalChannel
}

// Close closes the signal channel
func (signalChannel *SignalsForwarder) Close() error {
	signal.Stop(*signalChannel)
	*signalChannel <- nil
	close(*signalChannel)
	return nil
}

var iif = collections.IIf
