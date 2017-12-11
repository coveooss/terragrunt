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

	"gopkg.in/yaml.v2"

	"github.com/gruntwork-io/terragrunt/options"
)

// CommandShouldBeApproved returns true if the command to run should go through the approval process (expect style)
// It will return false if -auto-approve is set and if the command is not in the commands list of the config.
func CommandShouldBeApproved(command string, terragruntOptions *options.TerragruntOptions) bool {
	for _, terraformArg := range terragruntOptions.TerraformCliArgs {
		if terraformArg == "-auto-approve" {
			return false
		}
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

// RunCommandToApprove Runs a command with approval (expect style)
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
	Initialized bool
	Commands    []string
	Expect      []string
	Complete    []string
}

var approvalConfig ApprovalConfig

func getApprovalConfig(terragruntOptions *options.TerragruntOptions) (ApprovalConfig, error) {
	var configFileYaml []byte
	var err error
	if !approvalConfig.Initialized {
		if len(terragruntOptions.ApprovalConfigFile) > 0 {
			path, err := LookPath(terragruntOptions.ApprovalConfigFile, terragruntOptions.Env["PATH"])
			if err != nil {
				return approvalConfig, err
			}
			configFileYaml, err = ioutil.ReadFile(path)
			if err != nil {
				return approvalConfig, err
			}
		} else {
			configFileYaml = []byte(defaultApprovalConfig)
		}

		if err = yaml.Unmarshal(configFileYaml, &approvalConfig); err != nil {
			return approvalConfig, err
		}
		approvalConfig.Initialized = true
	}
	return approvalConfig, nil
}

const defaultApprovalConfig = `commands: 
  - apply
complete: 
  - "Apply complete!"
  - "Apply cancelled."
  - "Error:"
expect: 
  - "Do you want to perform these actions"
`
