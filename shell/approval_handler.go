package shell

import (
	"bufio"
	goErrors "errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"gopkg.in/yaml.v2"
)

// CommandShouldBeApproved returns true if the command to run should go through the approval process (expect style)
// It will return false if -auto-approve is set and if the command is not in the commands list of the config.
func CommandShouldBeApproved(command string, terragruntOptions *options.TerragruntOptions) bool {
	if util.ListContainsElement(terragruntOptions.TerraformCliArgs, "-auto-approve") {
		return false
	}
	config, err := getApprovalConfig(terragruntOptions)
	if err != nil {
		panic(err)
	}

	for _, commandToApprove := range config.Commands {
		if strings.Contains(command, commandToApprove) {
			return true
		}
	}
	return false
}

var sharedMutex = sync.Mutex{}

// RunCommandToApprove runs a command with approval (expect style)
func RunCommandToApprove(cmd *exec.Cmd, terragruntOptions *options.TerragruntOptions) error {
	sharedMutex.Lock()
	defer sharedMutex.Unlock()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdOutInterceptor := newOutputInterceptor(cmd.Stdout, terragruntOptions)
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
		return goErrors.New("Waited 30 seconds for input prompt. Did not get it.")
	}

	if isDashAllQuery(terragruntOptions.Env["TERRAGRUNT_ARGS"]) {
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
		return goErrors.New("Terraform did not complete successfully.")
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
	subWriter          io.Writer
	buffer             []byte
	expectStatements   []string
	completeStatements []string
}

func newOutputInterceptor(subWriter io.Writer, terragruntOptions *options.TerragruntOptions) *OutputInterceptor {
	config, err := getApprovalConfig(terragruntOptions)
	if err != nil {
		panic(err)
	}

	return &OutputInterceptor{
		subWriter:          subWriter,
		expectStatements:   config.Expect,
		completeStatements: config.Complete,
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
	return interceptor.IsComplete() || interceptor.bufferContainsString(interceptor.expectStatements)
}

// IsComplete returns true if the command is complete and should exit.
func (interceptor *OutputInterceptor) IsComplete() bool {
	return interceptor.bufferContainsString(interceptor.completeStatements)
}

func (interceptor *OutputInterceptor) bufferContainsString(listOfStrings []string) bool {
	for _, str := range listOfStrings {
		if strings.Contains(interceptor.GetBuffer(), str) {
			return true
		}
	}
	return false
}

// ApprovalConfig represents the config file that can be provided to modify the approval process. See the default config below for an example.
type ApprovalConfig struct {
	Commands []string
	Expect   []string
	Complete []string
}

func getApprovalConfig(terragruntOptions *options.TerragruntOptions) (config *ApprovalConfig, err error) {
	if approvalConfig != nil {
		return approvalConfig, nil
	}

	if terragruntOptions.ApprovalConfigFile != "" {
		var (
			path    string
			content []byte
		)

		if path, err = LookPath(terragruntOptions.ApprovalConfigFile, terragruntOptions.Env["PATH"]); err != nil {
			return
		}

		if content, err = ioutil.ReadFile(path); err != nil {
			return
		}
		if err = yaml.Unmarshal(content, config); err != nil {
			config = approvalConfig
			return
		}
	} else {
		config = &ApprovalConfig{
			[]string{"apply"},
			[]string{"Apply complete!", "Apply cancelled.", "Error:"},
			[]string{"Do you want to perform these actions"},
		}
	}
	approvalConfig = config
	return
}

var approvalConfig *ApprovalConfig
