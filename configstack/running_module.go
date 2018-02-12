package configstack

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
)

// Represents the status of a module that we are trying to apply as part of the apply-all or destroy-all command
type ModuleStatus int

const NORMAL_EXIT_CODE = 0
const ERROR_EXIT_CODE = 1
const UNDEFINED_EXIT_CODE = -1

const (
	Waiting ModuleStatus = iota
	Running
	Finished
)

// Using CreateMultiErrors as a variable instead of a function allows us to override the function used to compose multi error object.
// It is used if a command wants to change the default behavior of the severity analysis that is implemented by default.
var CreateMultiErrors = func(errs []error) error {
	return MultiError{Errors: errs}
}

// Represents a module we are trying to "run" (i.e. apply or destroy) as part of the apply-all or destroy-all command
type runningModule struct {
	Module         *TerraformModule
	Status         ModuleStatus
	Err            error
	DependencyDone chan *runningModule
	Dependencies   map[string]*runningModule
	NotifyWhenDone []*runningModule
	OutStream      bytes.Buffer
	Writer         io.Writer
	Handler        ModuleHandler
	Mutex          *sync.Mutex // A shared mutex pointer to ensure that there is no concurrency problem when job finish and report

	bufferIndex int // Indicates the position of the buffer that has been flushed to the logger
}

// This controls in what order dependencies should be enforced between modules
type DependencyOrder int

const (
	NormalOrder DependencyOrder = iota
	ReverseOrder
)

// Handler function prototype to inject interaction during the processing.
// The function receive the current module, its output and its error in parameter.
// Normally, the handler should return the same error as received in parameter, but it is possible to
// alter the normal course of the proccess by changing the error result.
type ModuleHandler func(TerraformModule, string, error) (string, error)

// Create a new RunningModule struct for the given module. This will initialize all fields to reasonable defaults,
// except for the Dependencies and NotifyWhenDone, both of which will be empty. You should fill these using a
// function such as crossLinkDependencies.
func newRunningModule(module *TerraformModule, mutex *sync.Mutex) *runningModule {
	return &runningModule{
		Module:         module,
		Status:         Waiting,
		DependencyDone: make(chan *runningModule, 1000), // Use a huge buffer to ensure senders are never blocked
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
		Writer:         module.TerragruntOptions.Writer,
		Mutex:          mutex,
	}
}

// Run the given map of module path to runningModule. To "run" a module, execute the RunTerragrunt command in its
// TerragruntOptions object. The modules will be executed in an order determined by their inter-dependencies, using
// as much concurrency as possible.
func RunModules(modules []*TerraformModule) error {
	return RunModulesWithHandler(modules, nil, NormalOrder)
}

// Run the given map of module path to runningModule. To "run" a module, execute the RunTerragrunt command in its
// TerragruntOptions object. The modules will be executed in the reverse order of their inter-dependencies, using
// as much concurrency as possible.
func RunModulesReverseOrder(modules []*TerraformModule) error {
	return RunModulesWithHandler(modules, nil, ReverseOrder)
}

// Run the given map of module path to runningModule. To "run" a module, execute the RunTerragrunt command in its
// TerragruntOptions object. The modules will be executed in an order determined by their inter-dependencies, using
// as much concurrency as possible.
// This version accepts a function as parameter (see: ModuleHander). The handler is called when the command is
// completed (either succeeded or failed).
func RunModulesWithHandler(modules []*TerraformModule, handler ModuleHandler, order DependencyOrder) error {
	runningModules, err := toRunningModules(modules, order)
	if err != nil {
		return err
	}

	var waitGroup sync.WaitGroup
	initSlowDown() // starts mechanism that control the maximum number of threads launched simultaneously

	for _, module := range runningModules {
		waitGroup.Add(1)
		module.Handler = handler
		go func(module *runningModule) {
			defer waitGroup.Done()
			logCatcher := util.LogCatcher{
				Writer: &module.OutStream,
				Logger: module.Module.TerragruntOptions.Logger,
			}
			module.Module.TerragruntOptions.Writer = logCatcher
			module.Module.TerragruntOptions.ErrWriter = logCatcher
			var completed bool
			go module.OutputPeriodicLogs(&completed) // Flush the output buffers periodically to confirm that the process is still alive
			module.runModuleWhenReady()
		}(module)
	}

	waitGroup.Wait()

	return collectErrors(runningModules)
}

// Convert the list of modules to a map from module path to a runningModule struct. This struct contains information
// about executing the module, such as whether it has finished running or not and any errors that happened. Note that
// this does NOT actually run the module. For that, see the RunModules method.
func toRunningModules(modules []*TerraformModule, dependencyOrder DependencyOrder) (map[string]*runningModule, error) {
	var mutex sync.Mutex

	runningModules := map[string]*runningModule{}
	for _, module := range modules {
		runningModules[module.Path] = newRunningModule(module, &mutex)
	}

	return crossLinkDependencies(runningModules, dependencyOrder)
}

// Loop through the map of runningModules and for each module M:
//
// * If dependencyOrder is NormalOrder, plug in all the modules M depends on into the Dependencies field and all the
//   modules that depend on M into the NotifyWhenDone field.
// * If dependencyOrder is ReverseOrder, do the reverse.
func crossLinkDependencies(modules map[string]*runningModule, dependencyOrder DependencyOrder) (map[string]*runningModule, error) {
	for _, module := range modules {
		for _, dependency := range module.Module.Dependencies {
			runningDependency, hasDependency := modules[dependency.Path]
			if !hasDependency {
				return modules, errors.WithStackTrace(DependencyNotFoundWhileCrossLinking{module, dependency})
			}
			if dependencyOrder == NormalOrder {
				module.Dependencies[runningDependency.Module.Path] = runningDependency
				runningDependency.NotifyWhenDone = append(runningDependency.NotifyWhenDone, module)
			} else {
				runningDependency.Dependencies[module.Module.Path] = module
				module.NotifyWhenDone = append(module.NotifyWhenDone, runningDependency)
			}
		}
	}

	return modules, nil
}

// Collect the errors from the given modules and return a single error object to represent them, or nil if no errors
// occurred
func collectErrors(modules map[string]*runningModule) error {
	errs := []error{}
	for _, module := range modules {
		if module.Err != nil {
			errs = append(errs, module.Err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.WithStackTrace(CreateMultiErrors(errs))
}

// Run a module once all of its dependencies have finished executing.
func (module *runningModule) dependencies() []string {
	result := make([]string, 0, len(module.Dependencies))
	for _, dep := range module.Dependencies {
		result = append(result, util.GetPathRelativeToWorkingDirMax(dep.Module.Path, 3))
	}
	return result
}

// Run a module once all of its dependencies have finished executing.
func (module *runningModule) runModuleWhenReady() {
	err := module.waitForDependencies()
	if err == nil {
		slowDown() // Just avoid starting all threads at the same moment
		err = module.runNow()
	}
	module.moduleFinished(err)
}

// Wait for all of this modules dependencies to finish executing. Return an error if any of those dependencies complete
// with an error. Return immediately if this module has no dependencies.
func (module *runningModule) waitForDependencies() error {
	log := module.Module.TerragruntOptions.Logger
	if len(module.Dependencies) > 0 {
		path := util.GetPathRelativeToWorkingDirMax(module.Module.Path, 3)
		log.Debugf("Module %s must wait for %s to finish", path, strings.Join(module.dependencies(), ", "))
	}
	for len(module.Dependencies) > 0 {
		doneDependency := <-module.DependencyDone
		delete(module.Dependencies, doneDependency.Module.Path)

		path := util.GetPathRelativeToWorkingDirMax(module.Module.Path, 3)
		depPath := util.GetPathRelativeToWorkingDirMax(doneDependency.Module.Path, 3)

		if doneDependency.Err != nil {
			if module.Module.TerragruntOptions.IgnoreDependencyErrors {
				log.Warningf("Dependency %[1]s of module %[2]s just finished with an error. Module %[2]s will have to return an error too. However, because of --terragrunt-ignore-dependency-errors, module %[2]s will run anyway.", depPath, path)
			} else {
				log.Warningf("Dependency %[1]s of module %[2]s just finished with an error. Module %[2]s will have to return an error too.", depPath, path)
				return DependencyFinishedWithError{module.Module, doneDependency.Module, doneDependency.Err}
			}
		} else {
			var moreDependencies string
			if len(module.Dependencies) > 0 {
				moreDependencies = fmt.Sprintf(" Module %s must still wait for %s.", path, strings.Join(module.dependencies(), ", "))
			}
			log.Debugf("Dependency %s of module %s just finished successfully.%s", depPath, path, moreDependencies)
		}
	}

	return nil
}

// Run a module right now by executing the RunTerragrunt command of its TerragruntOptions field.
func (module *runningModule) runNow() error {
	module.Status = Running

	if module.Module.AssumeAlreadyApplied {
		module.Module.TerragruntOptions.Logger.Debugf("Assuming module %s has already been applied and skipping it", module.Module.Path)
		return nil
	}
	module.Module.TerragruntOptions.Logger.Debugf("Running module %s now", util.GetPathRelativeToWorkingDirMax(module.Module.Path, 3))
	return module.Module.TerragruntOptions.RunTerragrunt(module.Module.TerragruntOptions)
}

var separator = strings.Repeat("-", 132)

// Record that a module has finished executing and notify all of this module's dependencies
func (module *runningModule) moduleFinished(moduleErr error) {
	status := "successfully!"
	logFinish := module.Module.TerragruntOptions.Logger.Infof
	output := module.OutStream.String()

	if module.Handler != nil {
		output, moduleErr = module.Handler(*module.Module, output, moduleErr)
	}

	if moduleErr != nil {
		status = fmt.Sprintf("with an error: %v", moduleErr)
		logFinish = module.Module.TerragruntOptions.Logger.Errorf
	}

	module.Mutex.Lock()
	defer module.Mutex.Unlock()
	logFinish("Module %s has finished %s", module.Module.Path, status)

	if output == "" {
		module.Module.TerragruntOptions.Logger.Info("No output")
	} else {
		fmt.Fprintln(module.Writer, color.HiGreenString("%s\n%v\n", separator, util.GetPathRelativeToWorkingDir(module.Module.Path)))
		fmt.Fprintln(module.Writer, output)
	}

	module.Status = Finished
	module.Err = moduleErr

	for _, toNotify := range module.NotifyWhenDone {
		toNotify.DependencyDone <- module
	}
}

// Custom error types

// DependencyFinishedWithError represents an error coming from a dependency
type DependencyFinishedWithError struct {
	Module     *TerraformModule
	Dependency *TerraformModule
	Err        error
}

func (e DependencyFinishedWithError) Error() string {
	return fmt.Sprintf("Cannot process module %s because one of its dependencies, %s, finished with an error: %s", e.Module, e.Dependency, e.Err)
}

func (e DependencyFinishedWithError) ExitStatus() (int, error) {
	if exitCode, err := shell.GetExitCode(e.Err); err == nil {
		return exitCode, nil
	}
	return -1, e
}

type MultiError struct {
	Errors []error
}

func (e MultiError) Error() string {
	errorStrings := []string{}
	for _, err := range e.Errors {
		errorStrings = append(errorStrings, err.Error())
	}
	return fmt.Sprintf("Encountered the following errors:\n%s", strings.Join(errorStrings, "\n"))
}

func (e MultiError) ExitStatus() (int, error) {
	exitCode := NORMAL_EXIT_CODE
	for i := range e.Errors {
		if code, err := shell.GetExitCode(e.Errors[i]); err != nil {
			return UNDEFINED_EXIT_CODE, e
		} else if code > exitCode {
			exitCode = code
		}
	}
	return exitCode, nil
}

type DependencyNotFoundWhileCrossLinking struct {
	Module     *runningModule
	Dependency *TerraformModule
}

func (err DependencyNotFoundWhileCrossLinking) Error() string {
	return fmt.Sprintf("Module %v specifies a dependency on module %v, but could not find that module while cross-linking dependencies. This is most likely a bug in Terragrunt. Please report it.", err.Module, err.Dependency)
}
