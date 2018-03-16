package options

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coveo/gotemplate/hcl"
	"github.com/coveo/gotemplate/utils"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

// TerragruntOptions represents options that configure the behavior of the Terragrunt program
type TerragruntOptions struct {
	// Location of the Terragrunt config file
	TerragruntConfigPath string

	// Location of the terraform binary
	TerraformPath string

	// Whether we should prompt the user for confirmation or always assume "yes"
	NonInteractive bool

	// The approval handler that will be used in a non interactive context
	ApprovalHandler string

	// CLI args that are intended for Terraform (i.e. all the CLI args except the --terragrunt ones)
	TerraformCliArgs []string

	// The working directory in which to run Terraform
	WorkingDir string

	// The AWS profile to use if specified (default = "")
	AwsProfile string

	// The logger to use for all logging
	Logger *logging.Logger

	// Environment variables at runtime
	Env map[string]string

	// Terraform variables at runtime
	Variables map[string]Variable

	// Download Terraform configurations from the specified source location into a temporary folder and run
	// Terraform in that temporary folder
	Source string

	// If set to true, delete the contents of the temporary folder before downloading Terraform source code into it
	SourceUpdate bool

	// Download Terraform configurations specified in the Source parameter into this folder
	DownloadDir string

	// If set to true, continue running *-all commands even if a dependency has errors. This is mostly useful for 'output-all <some_variable>'. See https://github.com/gruntwork-io/terragrunt/issues/193
	IgnoreDependencyErrors bool

	// If you want stdout to go somewhere other than os.stdout
	Writer io.Writer

	// If you want stderr to go somewhere other than os.stderr
	ErrWriter io.Writer

	// A command that can be used to run Terragrunt with the given options. This is useful for running Terragrunt
	// multiple times (e.g. when spinning up a stack of Terraform modules). The actual command is normally defined
	// in the cli package, which depends on almost all other packages, so we declare it here so that other
	// packages can use the command without a direct reference back to the cli package (which would create a
	// circular dependency).
	RunTerragrunt func(*TerragruntOptions) error

	// If set in terragrunt configuration, this string is added to the directory name before calculating the hashing
	// This allow differentiation based on certain attribute to ensure that different config (env, region) are executed
	// in distinct folder
	Uniqueness string

	// If set, this indicate that remaining interpolation are not considered as an error during the configuration
	// resolving process. This allows further resolution of variables that are not initially defined.
	IgnoreRemainingInterpolation bool

	// Indicates the maximum wait time before flushing the output of a background job
	RefreshOutputDelay time.Duration

	// Indicates the number of concurrent workers
	NbWorkers int

	// The list of files (should be only one) where to save files if save_variables() has been invoked by the user
	deferredSaveList map[string]bool
}

// NewTerragruntOptions creates a new TerragruntOptions object with reasonable defaults for real usage
func NewTerragruntOptions(terragruntConfigPath string) *TerragruntOptions {
	workingDir := filepath.Dir(terragruntConfigPath)

	downloadDir := util.GetTempDownloadFolder("terragrunt")
	// On some versions of Windows, the default temp dir is a fairly long path (e.g. C:/Users/JONDOE~1/AppData/Local/Temp/2/).
	// This is a problem because Windows also limits path lengths to 260 characters, and with nested folders and hashed folder names
	// (e.g. from running terraform get), you can hit that limit pretty quickly. Therefore, we try to set the temporary download
	// folder to something slightly shorter, but still reasonable.
	if runtime.GOOS == "windows" {
		downloadDir = `C:\\Windows\\Temp\\terragrunt`
	}

	return &TerragruntOptions{
		TerragruntConfigPath: terragruntConfigPath,
		TerraformPath:        "terraform",
		TerraformCliArgs:     []string{},
		WorkingDir:           workingDir,
		Logger:               util.CreateLogger(""),
		Env:                  make(map[string]string),
		Variables:            make(map[string]Variable),
		DownloadDir:          downloadDir,
		Writer:               os.Stdout,
		ErrWriter:            os.Stderr,
		RunTerragrunt: func(terragruntOptions *TerragruntOptions) error {
			return errors.WithStackTrace(ErrRunTerragruntCommandNotSet)
		},
	}
}

// NewTerragruntOptionsForTest creates a new TerragruntOptions object with reasonable defaults for test usage
func NewTerragruntOptionsForTest(terragruntConfigPath string) *TerragruntOptions {
	opts := NewTerragruntOptions(terragruntConfigPath)
	opts.NonInteractive = true
	return opts
}

// Clone creates a copy of this TerragruntOptions, but with different values for the given variables. This is useful for
// creating a TerragruntOptions that behaves the same way, but is used for a Terraform module in a different folder.
func (terragruntOptions TerragruntOptions) Clone(terragruntConfigPath string) *TerragruntOptions {
	newOptions := terragruntOptions
	newOptions.TerragruntConfigPath = terragruntConfigPath
	newOptions.WorkingDir = filepath.Dir(terragruntConfigPath)
	newOptions.Logger = util.CreateLogger(util.GetPathRelativeToWorkingDir(newOptions.WorkingDir))
	newOptions.Env = make(map[string]string, len(terragruntOptions.Env))
	newOptions.Variables = make(map[string]Variable, len(terragruntOptions.Variables))

	// We create a distinct map for the environment variables
	for key, value := range terragruntOptions.Env {
		newOptions.Env[key] = value
	}

	// We do a deep copy of the variables since they must be distinct from the original
	for key, value := range terragruntOptions.Variables {
		newOptions.SetVariable(key, value.Value, value.Source)
	}
	return &newOptions
}

// SetStatus saves environment variables indicating the current execution status
func (terragruntOptions *TerragruntOptions) SetStatus(exitCode int, err error) {
	errorMessage := fmt.Sprint(err)
	if err == nil {
		errorMessage = ""
	}
	terragruntOptions.Env[EnvLastError] = errorMessage
	terragruntOptions.Env[EnvLastStatus] = fmt.Sprint(exitCode)

	currentStatus := terragruntOptions.Env[EnvStatus]
	if currentStatus == "0" || currentStatus == "" {
		terragruntOptions.Env[EnvStatus] = fmt.Sprint(exitCode)
	}
	if err != nil {
		errors := append(strings.Split(terragruntOptions.Env[EnvError], "\n"), errorMessage)
		terragruntOptions.Env[EnvError] = strings.Join(util.RemoveElementFromList(errors, ""), "\n")
	}
}

// GetContext returns the current context from the variables set
func (terragruntOptions TerragruntOptions) GetContext() (result map[string]interface{}) {
	result = make(map[string]interface{}, len(terragruntOptions.Variables))
	for key, value := range terragruntOptions.Variables {
		result[key] = value.Value
	}
	return
}

// SaveVariables - Actually save the variables to the list of deferred files
func (terragruntOptions *TerragruntOptions) SaveVariables() (err error) {
	if terragruntOptions.deferredSaveList != nil {
		variables := terragruntOptions.GetContext()

		for file := range terragruntOptions.deferredSaveList {
			terragruntOptions.Logger.Debug("Saving variables into", file)
			var content []byte
			switch strings.ToLower(filepath.Ext(file)) {
			case ".yml", ".yaml":
				content, err = yaml.Marshal(variables)
			case ".tfvars":
				content, err = hcl.MarshalTFVarsIndent(variables, "", "  ")
			case ".hcl":
				content, err = hcl.MarshalIndent(variables, "", "  ")
			default:
				content, err = json.MarshalIndent(variables, "", "  ")
			}
			if err != nil {
				return
			}
			if content[len(content)-1] != '\n' {
				content = append(content, '\n')
			}
			err = ioutil.WriteFile(filepath.Join(terragruntOptions.WorkingDir, file), content, 0644)
		}
	}
	return
}

// ImportVariablesFromFile loads variables from the file indicated by path
func (terragruntOptions *TerragruntOptions) ImportVariablesFromFile(path string, origin VariableSource) error {
	if !strings.Contains(path, "/") {
		path = util.JoinPath(terragruntOptions.WorkingDir, path)
	}
	vars, err := util.LoadVariablesFromFile(path, terragruntOptions.WorkingDir, terragruntOptions.GetContext())
	if err != nil {
		return err
	}
	terragruntOptions.importVariables(vars, origin)
	return nil

}

// ImportVariables load variables from the content, source indicates the path from where the content has been loaded
func (terragruntOptions *TerragruntOptions) ImportVariables(content string, source string, origin VariableSource) error {
	vars, err := util.LoadVariablesFromSource(content, source, terragruntOptions.WorkingDir, terragruntOptions.GetContext())
	if err != nil {
		return err
	}
	terragruntOptions.importVariables(vars, origin)
	return nil
}

func (terragruntOptions *TerragruntOptions) importVariables(vars map[string]interface{}, origin VariableSource) {
	for key, value := range vars {
		if key == "terragrunt" {
			// We do not import the terragrunt variable
			continue
		}
		terragruntOptions.SetVariable(key, value, origin)
	}
}

// AddDeferredSaveVariables - Add a path where to save the variable list
func (terragruntOptions *TerragruntOptions) AddDeferredSaveVariables(filename string) {
	if terragruntOptions.deferredSaveList == nil {
		terragruntOptions.deferredSaveList = map[string]bool{}
	}
	terragruntOptions.deferredSaveList[filename] = true
}

// VariablesExplicitlyProvided returns the list of variables that have been explicitly provided as argument
func (terragruntOptions *TerragruntOptions) VariablesExplicitlyProvided() (result []string) {
	for key, arg := range terragruntOptions.Variables {
		if arg.Source == VarParameterExplicit {
			result = append(result, key)
		}
	}
	return
}

// EnvironmentVariables returns an array of string that defines the environment variables as required
// by external commands
func (terragruntOptions *TerragruntOptions) EnvironmentVariables() (result []string) {
	for key, value := range terragruntOptions.Env {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}
	return
}

// Println uses the embedded writer to print
func (terragruntOptions *TerragruntOptions) Println(args ...interface{}) (n int, err error) {
	return fmt.Fprintln(terragruntOptions.Writer, args...)
}

// Printf uses the embedded writer to print
func (terragruntOptions *TerragruntOptions) Printf(format string, args ...interface{}) (n int, err error) {
	return fmt.Fprintf(terragruntOptions.Writer, format, args...)
}

// SetVariable overwrites the value in the variables map only if the source is more significant than the original value
func (terragruntOptions *TerragruntOptions) SetVariable(key string, value interface{}, source VariableSource) {
	if strings.Contains(key, ".") {
		keys := strings.Split(key, ".")
		key = keys[0]
		value = util.ConvertToMap(value, keys[1:]...)
	}
	target := terragruntOptions.Variables[key]
	oldMap, oldIsMap := target.Value.(map[string]interface{})
	newMap, newIsMap := value.(map[string]interface{})

	if oldIsMap && newIsMap {
		// Map variables have a special treatment since we merge them instead of simply overwriting them

		if source > target.Source || target.Source == source && source != ConfigVarFile {
			// Values defined at the same level overwrite the previous values except for those defined in config file
			oldMap, newMap = newMap, oldMap
		}
		newValue, err := utils.MergeMaps(oldMap, newMap)
		if err != nil {
			terragruntOptions.Logger.Warningf("Unable to merge variable %s: %v and %v", key, target.Value, value)
		} else {
			terragruntOptions.Variables[key] = Variable{source, newValue}
			return
		}
	} else if target.Value != nil && (oldIsMap || newIsMap) {
		terragruntOptions.Logger.Warningf("Different types for %s: %v vs %v", key, target.Value, value)
	}

	if target.Source <= source {
		// We only override value if the source has less or equal precedence than the previous value
		if source == ConfigVarFile && target.Source == ConfigVarFile {
			// Values defined at the same level overwrite the previous values except for those defined in config file
			return
		}
		if target.Source != UndefinedSource {
			terragruntOptions.Logger.Infof("Overwriting value for %s with %v", key, value)
		}
		terragruntOptions.Variables[key] = Variable{source, value}
	}
}

// Variable defines value and origin of a variable (origin is important due to the precedence of the definition)
// i.e. A value specified by -var has precedence over value defined in -var-file
type Variable struct {
	Source VariableSource
	Value  interface{}
}

// VariableSource is an enum defining the priority of the source for variable definition
type VariableSource byte

// The order indicates precedence (latest have the highest priority)
const (
	UndefinedSource VariableSource = iota
	Default
	ConfigVarFile
	VarFile
	VarFileExplicit
	VarParameter
	Environment
	VarParameterExplicit
)

// ErrRunTerragruntCommandNotSet is a custom error
var ErrRunTerragruntCommandNotSet = fmt.Errorf("The RunTerragrunt option has not been set on this TerragruntOptions object")
