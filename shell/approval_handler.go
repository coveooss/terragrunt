package shell

import (
	"bufio"
	"bytes"
	goErrors "errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gruntwork-io/terragrunt/options"
)

var commandsWithApprovals = [...]string{"readable-apply"}

func CommandShouldBeApproved(command string) bool {
	for _, commandToApprove := range commandsWithApprovals {
		if strings.Contains(command, commandToApprove) {
			return true
		}
	}
	return false
}

func RunCommandToApprove(cmd *exec.Cmd, terragruntOptions *options.TerragruntOptions) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdOutInterceptor := newOutputInterceptor(cmd.Stdout)
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
		return goErrors.New("Waited 10 seconds for input prompt. Did not get it.")
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

	// Not checked for error since a refusal will return a non-zero status
	cmd.Wait()

	if _, ok := cmd.Stdout.(*bytes.Buffer); ok {
		clearCmd := exec.Command("clear")
		clearCmd.Stdout = os.Stdout
		err = clearCmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
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

type OutputInterceptor struct {
	subWriter io.Writer
	buffer    []byte
}

func newOutputInterceptor(subWriter io.Writer) *OutputInterceptor {
	return &OutputInterceptor{
		subWriter: subWriter,
	}
}

func (interceptor *OutputInterceptor) Write(p []byte) (n int, err error) {
	interceptor.buffer = append(interceptor.buffer, p...)
	return interceptor.subWriter.Write(p)
}

func (interceptor *OutputInterceptor) GetBuffer() string {
	return string(interceptor.buffer)
}

func (interceptor *OutputInterceptor) WaitingForValue() bool {
	return strings.Contains(interceptor.GetBuffer(), "Do you want to perform these actions") && !interceptor.IsComplete()
}

func (interceptor *OutputInterceptor) IsComplete() bool {
	return strings.Contains(interceptor.GetBuffer(), "Apply complete!") || strings.Contains(interceptor.GetBuffer(), "Apply cancelled.")
}
