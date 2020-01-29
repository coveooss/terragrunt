package shell

import (
	"bufio"
	goErrors "errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gruntwork-io/terragrunt/options"
)

var sharedMutex = sync.Mutex{}

// RunCommandToApprove runs a command with approval (expect style)
func RunCommandToApprove(cmd *exec.Cmd, expectedStatements []string, completedStatements []string, terragruntOptions *options.TerragruntOptions) error {
	sharedMutex.Lock()
	defer sharedMutex.Unlock()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdOutInterceptor := newOutputInterceptor(cmd.Stdout, expectedStatements, completedStatements)
	cmd.Stdout = stdOutInterceptor
	err = cmd.Start()
	if err != nil {
		return err
	}
	i := 0
	for !stdOutInterceptor.WaitingForValue() && i < 30 {
		time.Sleep(1 * time.Second)
		i++
	}
	if i == 30 {
		return goErrors.New("waited 30 seconds for input prompt. Did not get it")
	}

	if isDashAllQuery(terragruntOptions.Env[options.EnvArgs]) {
		fmt.Println(stdOutInterceptor.GetBuffer())
	}

	if !stdOutInterceptor.IsComplete() {
		var text string
		if len(terragruntOptions.ApprovalHandler) > 0 {
			text, err = approveWithCustomHandler(terragruntOptions, stdOutInterceptor.GetBuffer())
		} else {
			text, err = approveInConsole()
		}
		if err != nil {
			return err
		}
		io.WriteString(stdin, text)
	}
	err = stdin.Close()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return goErrors.New("terraform did not complete successfully")
	}

	return nil
}

func isDashAllQuery(terragruntArgs string) bool {
	return strings.Contains(terragruntArgs, "-all")
}

func approveInConsole() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	return reader.ReadString('\n')
}

func approveWithCustomHandler(terragruntOptions *options.TerragruntOptions, prompt string) (string, error) {
	command := strings.Split(terragruntOptions.ApprovalHandler, " ")[0]
	command, err := LookPath(command, terragruntOptions.Env["PATH"])
	if err != nil {
		return "", err
	}
	args := strings.Split(terragruntOptions.ApprovalHandler, " ")[1:]
	for index := range args {
		args[index] = strings.Replace(args[index], "{val}", prompt, -1)
	}
	approvalCmd := exec.Command(command, args...)
	approvalCmd.Stderr = os.Stderr
	resp, err := approvalCmd.Output()

	if err != nil {
		return "", err
	}
	return string(resp), nil
}

// OutputInterceptor intercepts all writes to a io.Writer and writes them to a buffer while still letting them pass through.
// Offers some functions to help the approval process.
type OutputInterceptor struct {
	subWriter           io.Writer
	buffer              []byte
	expectedStatements  []string
	completedStatements []string
}

func newOutputInterceptor(subWriter io.Writer, expectedStatements []string, completedStatements []string) *OutputInterceptor {
	return &OutputInterceptor{
		subWriter:           subWriter,
		expectedStatements:  expectedStatements,
		completedStatements: completedStatements,
	}
}

func (interceptor *OutputInterceptor) Write(p []byte) (n int, err error) {
	interceptor.buffer = append(interceptor.buffer, p...)
	return interceptor.subWriter.Write(p)
}

// GetBuffer returns the string value of all intercepted data to the underlying writer.
func (interceptor *OutputInterceptor) GetBuffer() string {
	return string(interceptor.buffer)
}

// WaitingForValue returns true if the command is waiting for an input value based on its output.
func (interceptor *OutputInterceptor) WaitingForValue() bool {
	return interceptor.IsComplete() || interceptor.bufferContainsString(interceptor.expectedStatements)
}

// IsComplete returns true if the command is complete and should exit.
func (interceptor *OutputInterceptor) IsComplete() bool {
	return interceptor.bufferContainsString(interceptor.completedStatements)
}

func (interceptor *OutputInterceptor) bufferContainsString(listOfStrings []string) bool {
	for _, str := range listOfStrings {
		if strings.Contains(interceptor.GetBuffer(), str) {
			return true
		}
	}
	return false
}
