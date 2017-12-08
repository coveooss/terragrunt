package cli

import (
	"sync"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

const getStackCommand = "get-stack"

// Get a list of terraform modules sorted by dependency order
func getStack(terragruntOptions *options.TerragruntOptions) (err error) {
	json := util.ListContainsElement(terragruntOptions.TerraformCliArgs, "json")
	run := util.ListContainsElement(terragruntOptions.TerraformCliArgs, "run")
	relative := !util.ListContainsElement(terragruntOptions.TerraformCliArgs, "abs") && !util.ListContainsElement(terragruntOptions.TerraformCliArgs, "absolute")

	var modules configstack.SimpleTerraformModules
	if run {
		if modules, err = getStackThroughExecution(terragruntOptions); err != nil {
			return
		}
	} else {
		stack, err := configstack.FindStackInSubfolders(terragruntOptions)
		if err != nil {
			return err
		}

		stack.SortModules()
		modules = stack.SimpleModules()
	}

	if relative {
		modules = modules.MakeRelative()
	}

	if json {
		terragruntOptions.Println(modules.JSON())
	} else {
		for _, module := range modules {
			terragruntOptions.Println(module.Path)
		}
	}

	return nil
}

// Get a list of terraform modules sorted by dependency order (but through real execution of the stack modules)
// Should give the same result as getStack
func getStackThroughExecution(terragruntOptions *options.TerragruntOptions) (modules configstack.SimpleTerraformModules, err error) {
	mutex := sync.Mutex{}
	runHandler = func(stackOptions *options.TerragruntOptions, stackConfig *config.TerragruntConfig) (err error) {
		mutex.Lock()
		defer mutex.Unlock()
		dependencies := make([]string, 0)
		if stackConfig.Dependencies != nil {
			for _, dep := range stackConfig.Dependencies.Paths {
				dependencies = append(dependencies, dep)
			}
		}
		modules = append(modules, configstack.SimpleTerraformModule{Path: stackOptions.WorkingDir, Dependencies: dependencies})
		return
	}

	err = runAll(getStackCommand, terragruntOptions)
	return
}
