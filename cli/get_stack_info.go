package cli

import (
	"encoding/json"
	"sync"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/options"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	yaml "gopkg.in/yaml.v2"
)

const getStackCommand = "get-stack"

// Get a list of terraform modules sorted by dependency order
func getStack(terragruntOptions *options.TerragruntOptions) (err error) {
	var (
		app      = kingpin.New("terragrunt get-stack", "Get stack detailed information")
		absolute bool
		modules  configstack.SimpleTerraformModules
	)

	run := app.Flag("run", "Run the full stack to get the result instead of just analysing the dependencies").Short('r').Bool()
	jsonOut := app.Flag("json", "Output result in JSON format").Short('j').Bool()
	yamlOut := app.Flag("yaml", "Output result in YAML format").Short('y').Bool()
	app.Flag("absolute", "Output absolute path (--abs)").Short('a').BoolVar(&absolute)
	app.Flag("abs", "").Hidden().BoolVar(&absolute)
	app.HelpFlag.Short('h')
	app.Parse(terragruntOptions.TerraformCliArgs[1:])

	if *run {
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

	if !absolute {
		modules = modules.MakeRelative()
	}

	if *jsonOut || *yamlOut {
		var result []byte
		var err error

		if *jsonOut {
			result, err = json.MarshalIndent(modules, "", "  ")
		} else {
			result, err = yaml.Marshal(modules)
		}
		if err != nil {
			panic(err)
		}
		terragruntOptions.Println(string(result))
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
