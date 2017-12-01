package cli

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

const getStackCommand = "get-stack"

// Get a list of terraform modules sorted by dependency order
func getStack(terragruntOptions *options.TerragruntOptions) error {
	json := util.ListContainsElement(terragruntOptions.TerraformCliArgs, "json")
	run := util.ListContainsElement(terragruntOptions.TerraformCliArgs, "run")

	if run {
		return getStackThroughExecution(terragruntOptions, json)
	}

	stack, err := configstack.FindStackInSubfolders(terragruntOptions)
	if err != nil {
		return err
	}
	stack.SortModules()

	if json {
		fmt.Println(stack.JSON())
	} else {
		for _, module := range stack.Modules {
			fmt.Println(util.GetPathRelativeToWorkingDir(module.Path))
		}
	}

	return nil
}

// Get a list of terraform modules sorted by dependency order (but through real execution of the stack modules)
// Should give the same result as getStack
func getStackThroughExecution(terragruntOptions *options.TerragruntOptions, json bool) (err error) {
	var stack configstack.SimpleTerraformModules

	mutex := sync.Mutex{}
	runHandler = func(stackOptions *options.TerragruntOptions, stackConfig *config.TerragruntConfig) error {
		mutex.Lock()
		defer mutex.Unlock()
		path := util.GetPathRelativeToWorkingDir(stackOptions.WorkingDir)
		dependencies := make([]string, 0)
		if stackConfig.Dependencies != nil {
			for _, dep := range stackConfig.Dependencies.Paths {
				dependencies = append(dependencies, util.GetPathRelativeToWorkingDir(filepath.Join(stackOptions.WorkingDir, dep)))
			}
		}
		stack = append(stack, configstack.SimpleTerraformModule{Path: path, Dependencies: dependencies})
		return nil
	}

	if err = runAll(getStackCommand, terragruntOptions); err == nil {
		if json {
			fmt.Println(stack.JSON())
		} else {
			for _, module := range stack {
				fmt.Println(module.Path)
			}
		}
	}
	return
}
