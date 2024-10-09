package cli

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/alecthomas/kingpin/v2"
	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/configstack"
	"github.com/coveooss/terragrunt/v2/options"

	yaml "gopkg.in/yaml.v3"
)

const getStackCommand = "get-stack"

// Get a list of terraform modules sorted by dependency order
func getStack(terragruntOptions *options.TerragruntOptions) (err error) {
	var (
		app      = kingpin.New("terragrunt get-stack", "Get stack detailed information")
		absolute bool
		modules  configstack.SimpleTerraformModules
	)

	run := app.Flag("run", "Run the full stack to get the result instead of just analyzing the dependencies").Short('r').Bool()
	output := app.Flag("output", "Specify format of the output (hcl, json, yaml)").Short('o').Enum("h", "hcl", "H", "HCL", "j", "json", "J", "JSON", "y", "yml", "yaml", "Y", "YML", "YAML")
	app.Flag("absolute", "Output absolute path (--abs)").Short('a').BoolVar(&absolute)
	app.Flag("abs", "").Hidden().BoolVar(&absolute)
	app.HelpFlag.Short('h')
	if _, err = app.Parse(terragruntOptions.TerraformCliArgs[1:]); err != nil {
		return
	}

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

	switch {
	case *output != "":
		var err error
		var result []byte
		switch strings.ToLower(*output) {
		case "h", "hcl":
			result, err = hcl.MarshalIndent(modules, "", "  ")
		case "j", "json":
			result, err = json.MarshalIndent(modules, "", "  ")
		case "y", "yml", "yaml":
			result, err = yaml.Marshal(modules)
		}
		if err != nil {
			panic(err)
		}
		terragruntOptions.Println(string(result))
	default:
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
			dependencies = append(dependencies, stackConfig.Dependencies.Paths...)
		}
		modules = append(modules, configstack.SimpleTerraformModule{Path: stackOptions.WorkingDir, Dependencies: dependencies})
		return
	}

	err = runAll(getStackCommand, terragruntOptions)
	return
}
