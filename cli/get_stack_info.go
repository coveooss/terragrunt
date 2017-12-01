package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

const CMD_GET_STACK = "get-stack"

func getStack(terragruntOptions *options.TerragruntOptions) (err error) {
	type stack struct {
		Path         string   `json:"path"`
		Dependencies []string `json:"dependencies"`
	}
	var stacks []stack

	mutex := sync.Mutex{}
	stackHandler = func(stackOptions *options.TerragruntOptions, stackConfig *config.TerragruntConfig) error {
		mutex.Lock()
		defer mutex.Unlock()
		path, _ := filepath.Rel(terragruntOptions.WorkingDir, stackOptions.WorkingDir)
		dependencies := make([]string, 0)
		if stackConfig.Dependencies != nil {
			for _, dep := range stackConfig.Dependencies.Paths {
				path, _ := filepath.Rel(terragruntOptions.WorkingDir, filepath.Join(stackOptions.WorkingDir, dep))
				dependencies = append(dependencies, path)
			}
		}
		stacks = append(stacks, stack{path, dependencies})
		return nil
	}

	if err = runAll(CMD_GET_STACK, terragruntOptions); err == nil {
		if util.ListContainsElement(terragruntOptions.TerraformCliArgs[1:], "json") {
			if json, err := json.MarshalIndent(stacks, "", "  "); err == nil {
				fmt.Println(string(json))
			}
		} else {
			for _, stack := range stacks {
				fmt.Println(stack.Path)
			}
		}
	}
	return
}

var stackHandler func(*options.TerragruntOptions, *config.TerragruntConfig) error
