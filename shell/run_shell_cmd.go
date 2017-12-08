package shell

import (
	"bytes"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"syscall"

	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/op/go-logging"
)

// Run the given Terraform command
func RunTerraformCommand(terragruntOptions *options.TerragruntOptions, args ...string) error {
	return RunShellCommand(terragruntOptions, terragruntOptions.TerraformPath, args...)
}

// Run the given Terraform command but redirect all outputs (both stdout and stderr) to the logger instead of
// the default stream. This allows us to isolate the true output of terraform command from the artefact of commands
// like init and get during the preparation steps.
// If the user redirect the stdout, he will only get the output for the terraform desired command.
func RunTerraformCommandAndRedirectOutputToLogger(terragruntOptions *options.TerragruntOptions, args ...string) error {
	output, err := RunShellCommandAndCaptureOutput(terragruntOptions, true, terragruntOptions.TerraformPath, args...)
	if err != nil {
		terragruntOptions.Logger.Error(output)
	} else {
		terragruntOptions.Logger.Info(output)
	}
	return err
}

// Run the given Terraform command and return the stdout as a string
func RunTerraformCommandAndCaptureOutput(terragruntOptions *options.TerragruntOptions, args ...string) (string, error) {
	return RunShellCommandAndCaptureOutput(terragruntOptions, false, terragruntOptions.TerraformPath, args...)
}

// Run the specified shell command with the specified arguments. Connect the command's stdin, stdout, and stderr to
// the currently running app.
func RunShellCommand(terragruntOptions *options.TerragruntOptions, command string, args ...string) error {
	return runShellCommand(terragruntOptions, false, command, args...)
}

// Run the specified shell command with the specified arguments. Connect the command's stdin, stdout, and stderr to
// the currently running app.
func RunShellCommandExpandArgs(terragruntOptions *options.TerragruntOptions, command string, args ...string) error {
	return runShellCommand(terragruntOptions, true, command, args...)
}

func runShellCommand(terragruntOptions *options.TerragruntOptions, expandArgs bool, command string, args ...string) error {
	logger := terragruntOptions.Logger.Notice
	if terragruntOptions.Writer != os.Stdout {
		logger = terragruntOptions.Logger.Info
	}
	logger("Running command:", command, strings.Join(args, " "))

	command, err := LookPath(command, terragruntOptions.Env["PATH"])
	if err != nil {
		return errors.WithStackTrace(err)
	}

	if expandArgs {
		args = util.ExpandArguments(args, terragruntOptions.WorkingDir)
	}

	cmd := exec.Command(command, args...)

	// TODO: consider adding prefix from terragruntOptions logger to stdout and stderr
	cmd.Stderr = os.Stderr
	cmd.Stdout = terragruntOptions.Writer

	if !strings.HasSuffix(command, "sh") || len(args) > 0 {
		// We do not redirect stderr if the actual command is a shell
		cmd.Stderr = terragruntOptions.ErrWriter
	}
	cmd.Env = terragruntOptions.EnvironmentVariables()

	// Terragrunt can run some commands (such as terraform remote config) before running the actual terraform
	// command requested by the user. The output of these other commands should not end up on stdout as this
	// breaks scripts relying on terraform's output.
	if !reflect.DeepEqual(terragruntOptions.TerraformCliArgs, args) {
		cmd.Stdout = cmd.Stderr
	}

	cmd.Dir = terragruntOptions.WorkingDir
	cmdChannel := make(chan error)

	signalChannel := NewSignalsForwarder(forwardSignals, cmd, terragruntOptions.Logger, cmdChannel)
	defer signalChannel.Close()

	if CommandShouldBeApproved(command) {
		err = RunCommandToApprove(cmd, terragruntOptions)
	} else {
		err = cmd.Run()
	}

	cmdChannel <- err
	return errors.WithStackTrace(err)
}

// Run the specified shell command with the specified arguments. Capture the command's stdout and return it as a
// string.
func RunShellCommandAndCaptureOutput(terragruntOptions *options.TerragruntOptions, copyWorkingDir bool, command string, args ...string) (string, error) {
	stdout := new(bytes.Buffer)

	terragruntOptionsCopy := terragruntOptions.Clone(terragruntOptions.TerragruntConfigPath)
	if copyWorkingDir {
		terragruntOptionsCopy.WorkingDir = terragruntOptions.WorkingDir
	}
	terragruntOptionsCopy.Writer = stdout
	terragruntOptionsCopy.ErrWriter = stdout

	// If the user specified -no-color, we should respect it in intermediate calls too
	const noColor = "-no-color"
	if util.ListContainsElement(terragruntOptions.TerraformCliArgs, noColor) {
		args = append(args, noColor)
	}

	err := RunShellCommand(terragruntOptionsCopy, command, args...)
	return stdout.String(), err
}

// LookPath search the supplied path to find the desired command
// It uses a mutex since it has to temporary override the global PATH variable.
func LookPath(command string, paths ...string) (string, error) {
	originalPath := os.Getenv("PATH")

	defer func() {
		os.Setenv("PATH", originalPath)
		lookPathMutex.Unlock()
	}()

	lookPathMutex.Lock()
	os.Setenv("PATH", strings.Join(paths, string(os.PathListSeparator)))
	return exec.LookPath(command)
}

var lookPathMutex sync.Mutex

// Return the exit code of a command. If the error does not implement errors.IErrorCode or is not an exec.ExitError type,
// the error is returned.
func GetExitCode(err error) (int, error) {
	if exiterr, ok := errors.Unwrap(err).(errors.IErrorCode); ok {
		return exiterr.ExitStatus()
	}

	if exiterr, ok := errors.Unwrap(err).(*exec.ExitError); ok {
		status := exiterr.Sys().(syscall.WaitStatus)
		return status.ExitStatus(), nil
	}
	return 0, err
}

type SignalsForwarder chan os.Signal

// Forwards signals to a command, waiting for the command to finish.
func NewSignalsForwarder(signals []os.Signal, c *exec.Cmd, logger *logging.Logger, cmdChannel chan error) SignalsForwarder {
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

func (signalChannel *SignalsForwarder) Close() error {
	signal.Stop(*signalChannel)
	*signalChannel <- nil
	close(*signalChannel)
	return nil
}
