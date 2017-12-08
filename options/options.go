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
	Variables VariableList

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

	// The list of files (should be only one) where to save files if save_variables() has been invoked by the user
	deferredSaveList map[string]bool
}

// Create a new TerragruntOptions object with reasonable defaults for real usage
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
		TerragruntConfigPath:   terragruntConfigPath,
		TerraformPath:          "terraform",
		NonInteractive:         false,
		TerraformCliArgs:       []string{},
		WorkingDir:             workingDir,
		Logger:                 util.CreateLogger(""),
		Env:                    map[string]string{},
		Variables:              VariableList{},
		Source:                 "",
		SourceUpdate:           false,
		DownloadDir:            downloadDir,
		IgnoreDependencyErrors: false,
		Writer:                 os.Stdout,
		ErrWriter:              os.Stderr,
		RunTerragrunt: func(terragruntOptions *TerragruntOptions) error {
			return errors.WithStackTrace(RunTerragruntCommandNotSet)
		},
	}
}

// Create a new TerragruntOptions object with reasonable defaults for test usage
func NewTerragruntOptionsForTest(terragruntConfigPath string) *TerragruntOptions {
	opts := NewTerragruntOptions(terragruntConfigPath)

	opts.NonInteractive = true

	return opts
}

// Create a copy of this TerragruntOptions, but with different values for the given variables. This is useful for
// creating a TerragruntOptions that behaves the same way, but is used for a Terraform module in a different folder.
func (terragruntOptions *TerragruntOptions) Clone(terragruntConfigPath string) *TerragruntOptions {
	workingDir := filepath.Dir(terragruntConfigPath)

	newOptions := TerragruntOptions{
		TerragruntConfigPath:         terragruntConfigPath,
		TerraformPath:                terragruntOptions.TerraformPath,
		NonInteractive:               terragruntOptions.NonInteractive,
		TerraformCliArgs:             terragruntOptions.TerraformCliArgs,
		WorkingDir:                   workingDir,
		Logger:                       util.CreateLogger(util.GetPathRelativeToWorkingDir(workingDir)),
		Env:                          map[string]string{},
		Variables:                    VariableList{},
		Source:                       terragruntOptions.Source,
		SourceUpdate:                 terragruntOptions.SourceUpdate,
		DownloadDir:                  terragruntOptions.DownloadDir,
		IgnoreDependencyErrors:       terragruntOptions.IgnoreDependencyErrors,
		Writer:                       terragruntOptions.Writer,
		ErrWriter:                    terragruntOptions.ErrWriter,
		RunTerragrunt:                terragruntOptions.RunTerragrunt,
		Uniqueness:                   terragruntOptions.Uniqueness,
		IgnoreRemainingInterpolation: terragruntOptions.IgnoreRemainingInterpolation,
		deferredSaveList:             terragruntOptions.deferredSaveList,
	}

	// We create a distinct map for the environment variables
	for key, value := range terragruntOptions.Env {
		newOptions.Env[key] = value
	}

	// We do a deep copy of the variables since they must be disctint from the original
	for key, value := range terragruntOptions.Variables {
		newOptions.Variables.SetValue(key, value.Value, value.Source)
	}
	return &newOptions
}

// SaveVariables - Actually save the variables to the list of deferred files
func (terragruntOptions *TerragruntOptions) SaveVariables() (err error) {
	if terragruntOptions.deferredSaveList != nil {
		variables := make(map[string]interface{}, len(terragruntOptions.Variables))

		// We keep only the value from the variable list, don't need the source
		for key, value := range terragruntOptions.Variables {
			variables[key] = value.Value
		}

		for file := range terragruntOptions.deferredSaveList {
			terragruntOptions.Logger.Debug("Saving variables into", file)
			var content []byte
			switch strings.ToLower(filepath.Ext(file)) {
			case ".yml", ".yaml":
				content, err = yaml.Marshal(variables)
				if err != nil {
					return
				}
			case ".tfvars", ".hcl":
				content = util.MarshalHCLVars(variables, 0)
			default:
				content, err = json.MarshalIndent(variables, "", "  ")
				if err != nil {
					return
				}
			}

			err = ioutil.WriteFile(filepath.Join(terragruntOptions.WorkingDir, file), content, 0644)
		}
	}
	return
}

// ImportVariablesFromFile loads variables from the file indicated by path
func (terragruntOptions *TerragruntOptions) ImportVariablesFromFile(path string, origin VariableSource) {
	vars, err := util.LoadVariablesFromFile(path)
	if err != nil {
		terragruntOptions.Logger.Errorf("Unable to import variables from %s, %v", path, err)
	}
	terragruntOptions.importVariables(vars, origin)
}

// ImportVariables load variables from the content, source indicates the path from where the content has been loaded
func (terragruntOptions *TerragruntOptions) ImportVariables(content string, source string, origin VariableSource) {
	vars, err := util.LoadVariables(content)
	if err != nil {
		terragruntOptions.Logger.Errorf("Unable to import variables from %s, %v", source, err)
	}
	terragruntOptions.importVariables(vars, origin)
}

// ImportVariables load variables from the content, source indicates the path from where the content has been loaded
func (terragruntOptions *TerragruntOptions) importVariables(vars map[string]interface{}, origin VariableSource) {
	for key, value := range vars {
		if key == "terragrunt" {
			// We do not import the terragrunt variable
			continue
		}
		terragruntOptions.Variables.SetValue(key, value, origin)
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

// VariableList defines the list of all variables defined during the processing of config files
type VariableList map[string]Variable

// SetValue overwrites the value in the variables map only if the source is more significant than the original value
func (variables VariableList) SetValue(key string, value interface{}, source VariableSource) {
	if variables[key].Source <= source {
		// We only override value if the source has less or equal precedence than the previous value
		if source == ConfigVarFile && variables[key].Source == ConfigVarFile {
			// Values defined in the lower config file have precedence to those defined in parents include
			return
		}
		variables[key] = Variable{source, value}
	}
}

type VariableSource byte

// Variable defines value and origin of a variable (origin is important due to the precedence of the definition)
// i.e. A value specified by -var has precedence over value defined in -var-file
type Variable struct {
	Source VariableSource
	Value  interface{}
}

// The order indicates precedence (latest have the highest priority)
const (
	UndefinedSource VariableSource = iota
	Default
	Environment
	ConfigVarFile
	VarFile
	VarFileExplicit
	VarParameter
	VarParameterExplicit
)

// Custom error types
var RunTerragruntCommandNotSet = fmt.Errorf("The RunTerragrunt option has not been set on this TerragruntOptions object")
