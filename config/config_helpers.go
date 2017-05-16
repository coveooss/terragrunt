package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

const GET_TEMP_FOLDER = "<TEMP_FOLDER>"

var INTERPOLATION_SYNTAX_REGEX = regexp.MustCompile(`\$\{.*?\}`)
var INTERPOLATION_SYNTAX_REGEX_SINGLE = regexp.MustCompile(`"\$\{.*?\}"`)
var HELPER_FUNCTION_SYNTAX_REGEX = regexp.MustCompile(`\$\{(.*?)\((.*?)\)\}`)
var HELPER_VAR_REGEX = regexp.MustCompile(`\$\{var\.([[[:alpha:]][\w-]*)\}`)
var HELPER_FUNCTION_GET_ENV_PARAMETERS_SYNTAX_REGEX = regexp.MustCompile(`\s*"(?P<env>[^=]+?)"\s*\,\s*(?:"(?P<default>.*?)"|var\.(?P<vardef>\w+))\s*`)
var HELPER_FUNCTION_SINGLE_STRING_PARAMETER_SYNTAX_REGEX = regexp.MustCompile(`\s*"(.*?)"\s*`)
var MAX_PARENT_FOLDERS_TO_CHECK = 100

// List of terraform commands that accept -lock-timeout
var TERRAFORM_COMMANDS_NEED_LOCKING = []string{
	"apply",
	"destroy",
	"import",
	"init",
	"plan",
	"refresh",
	"taint",
	"untaint",
}

// List of terraform commands that accept -var or -var-file
var TERRAFORM_COMMANDS_NEED_VARS = []string{
	"apply",
	"console",
	"destroy",
	"import",
	"plan",
	"push",
	"refresh",
}

type EnvVar struct {
	Name         string
	DefaultValue string
}

// Given a string value from a Terragrunt configuration, parse the string, resolve any calls to helper functions using
// the syntax ${...}, and return the final value.
func ResolveTerragruntConfigString(terragruntConfigString string, include IncludeConfig, terragruntOptions *options.TerragruntOptions) (resolved string, finalErr error) {
	// The function we pass to ReplaceAllStringFunc cannot return an error, so we have to use named error
	// parameters to capture such errors.

	resolved = INTERPOLATION_SYNTAX_REGEX_SINGLE.ReplaceAllStringFunc(terragruntConfigString, func(str string) string {
		// We do a special treatment for function returning an array of values because they need
		// to be declared like that "${function()" and the result must get rid of the surrounding quote
		// to return a real array of values.
		internalStr := str[1 : len(str)-1] // We get rid of the surrounding quote
		out, err := resolveTerragruntInterpolation(internalStr, include, terragruntOptions)
		if err != nil {
			finalErr = err
		}

		switch out := out.(type) {
		case []string:
			return util.ListToHCLArray(out)
		}

		// The function is not returning an array of string, so left the string unchanged
		// Remaining functions will be processed by the next statement
		return str
	})

	resolved = INTERPOLATION_SYNTAX_REGEX.ReplaceAllStringFunc(resolved, func(str string) string {
		out, err := resolveTerragruntInterpolation(str, include, terragruntOptions)
		if err != nil {
			finalErr = err
		}

		switch out := out.(type) {
		case string:
			return out
		default:
			return fmt.Sprintf("%v", out)
		}
	})

	return
}

// Substitute any variables in the string if there is a value associated with the variable
func SubstituteVars(str string, terragruntOptions *options.TerragruntOptions) string {
	if newStr, ok := resolveTerragruntVars(str, terragruntOptions); ok {
		return newStr
	}
	return str
}

// Resolve the references to variables ${var.name} if there are
func resolveTerragruntVars(str string, terragruntOptions *options.TerragruntOptions) (string, bool) {
	var match = false
	str = HELPER_VAR_REGEX.ReplaceAllStringFunc(str, func(str string) string {
		match = true
		matches := HELPER_VAR_REGEX.FindStringSubmatch(str)
		if found, ok := terragruntOptions.Variables[matches[1]]; ok {
			return fmt.Sprint(found.Value)
		}
		if terragruntOptions.EraseNonDefinedVariables {
			if !warningDone[matches[0]] {
				terragruntOptions.Logger.Warningf("Variable %s undefined", matches[0])
				warningDone[matches[0]] = true
			}
			return ""
		}
		return matches[0]
	})

	return str, match
}

var warningDone = map[string]bool{}

// Resolve a single call to an interpolation function of the format ${some_function()} of ${var.some_var} in a Terragrunt configuration
func resolveTerragruntInterpolation(str string, include IncludeConfig, terragruntOptions *options.TerragruntOptions) (interface{}, error) {
	if result, ok := resolveTerragruntVars(str, terragruntOptions); ok {
		return result, nil
	}

	matches := HELPER_FUNCTION_SYNTAX_REGEX.FindStringSubmatch(str)
	if len(matches) == 3 {
		return executeTerragruntHelperFunction(matches[1], matches[2], include, terragruntOptions)
	}

	return "", errors.WithStackTrace(InvalidInterpolationSyntax(str))
}

// Execute a single Terragrunt helper function and return its value as a string
func executeTerragruntHelperFunction(functionName string, parameters string, include IncludeConfig, terragruntOptions *options.TerragruntOptions) (interface{}, error) {
	switch functionName {
	case "find_in_parent_folders":
		return findInParentFolders(terragruntOptions)
	case "path_relative_to_include":
		return pathRelativeToInclude(include, terragruntOptions)
	case "path_relative_from_include":
		return pathRelativeFromInclude(include, terragruntOptions)
	case "get_env":
		return getEnvironmentVariable(parameters, terragruntOptions)
	case "get_current_dir":
		return filepath.Dir(include.Path), nil
	case "get_leaf_dir", "get_tfvars_dir":
		return getTfVarsDir(terragruntOptions)
	case "get_parent_dir", "get_parent_tfvars_dir":
		return getParentTfVarsDir(include, terragruntOptions)
	case "get_aws_account_id":
		return getAWSAccountID()
	case "save_variables":
		return saveVariables(parameters, terragruntOptions)
	case "get_terraform_commands_that_need_vars":
		return TERRAFORM_COMMANDS_NEED_VARS, nil
	case "get_terraform_commands_that_need_locking":
		return TERRAFORM_COMMANDS_NEED_LOCKING, nil
	case "get_temp_folder":
		return GET_TEMP_FOLDER, nil
	default:
		return "", errors.WithStackTrace(UnknownHelperFunction(functionName))
	}
}

// Return the directory where the Terragrunt configuration file lives
func getTfVarsDir(terragruntOptions *options.TerragruntOptions) (string, error) {
	terragruntConfigFileAbsPath, err := filepath.Abs(terragruntOptions.TerragruntConfigPath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return filepath.ToSlash(filepath.Dir(terragruntConfigFileAbsPath)), nil
}

// Return the parent directory where the Terragrunt configuration file lives
func getParentTfVarsDir(include IncludeConfig, terragruntOptions *options.TerragruntOptions) (string, error) {
	parentPath, err := pathRelativeFromInclude(include, terragruntOptions)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	currentPath := filepath.Dir(terragruntOptions.TerragruntConfigPath)
	parentPath, err = filepath.Abs(filepath.Join(currentPath, parentPath))
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return filepath.ToSlash(parentPath), nil
}

func parseGetEnvParameters(parameters string, terragruntOptions *options.TerragruntOptions) (EnvVar, error) {
	envVariable := EnvVar{}
	matches := HELPER_FUNCTION_GET_ENV_PARAMETERS_SYNTAX_REGEX.FindStringSubmatch(parameters)
	if len(matches) < 4 {
		return envVariable, errors.WithStackTrace(InvalidFunctionParameters(parameters))
	}

	for index, name := range HELPER_FUNCTION_GET_ENV_PARAMETERS_SYNTAX_REGEX.SubexpNames() {
		switch name {
		case "env":
			envVariable.Name = strings.TrimSpace(matches[index])
		case "default":
			if matches[index] != "" {
				envVariable.DefaultValue = strings.TrimSpace(matches[index])
			}
		case "vardef":
			if matches[index] != "" {
				if value, found := terragruntOptions.Variables[matches[index]]; found {
					envVariable.DefaultValue = fmt.Sprintf("%v", value.Value)
				} else {
					// The variable is not defined yet, we replace it by its name
					envVariable.DefaultValue = fmt.Sprintf("${var.%v}", matches[index])
				}
			}
		}
	}

	return envVariable, nil
}

func getEnvironmentVariable(parameters string, terragruntOptions *options.TerragruntOptions) (string, error) {
	parameterMap, err := parseGetEnvParameters(parameters, terragruntOptions)

	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	envValue, exists := terragruntOptions.Env[parameterMap.Name]

	if !exists {
		envValue = parameterMap.DefaultValue
	}

	return envValue, nil
}

// Find a parent Terragrunt configuration file in the parent folders above the current Terragrunt configuration file
// and return its path
func findInParentFolders(terragruntOptions *options.TerragruntOptions) (string, error) {
	previousDir, err := filepath.Abs(filepath.Dir(terragruntOptions.TerragruntConfigPath))
	previousDir = filepath.ToSlash(previousDir)

	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	// To avoid getting into an accidental infinite loop (e.g. do to cyclical symlinks), set a max on the number of
	// parent folders we'll check
	for i := 0; i < MAX_PARENT_FOLDERS_TO_CHECK; i++ {
		currentDir := filepath.ToSlash(filepath.Dir(previousDir))
		if currentDir == previousDir {
			return "", errors.WithStackTrace(ParentTerragruntConfigNotFound(terragruntOptions.TerragruntConfigPath))
		}

		configPath := DefaultConfigPath(currentDir)
		if util.FileExists(configPath) {
			return util.GetPathRelativeTo(configPath, filepath.Dir(terragruntOptions.TerragruntConfigPath))
		}

		previousDir = currentDir
	}

	return "", errors.WithStackTrace(CheckedTooManyParentFolders(terragruntOptions.TerragruntConfigPath))
}

// Return the relative path between the included Terragrunt configuration file and the current Terragrunt configuration
// file
func pathRelativeToInclude(include IncludeConfig, terragruntOptions *options.TerragruntOptions) (string, error) {
	parent := getparentLocalConfigFilesLocation(include, terragruntOptions)
	child := filepath.Dir(terragruntOptions.TerragruntConfigPath)
	return util.GetPathRelativeTo(child, parent)
}

// Return the relative path from the current Terragrunt configuration to the included Terragrunt configuration file
func pathRelativeFromInclude(include IncludeConfig, terragruntOptions *options.TerragruntOptions) (string, error) {
	parent := getparentLocalConfigFilesLocation(include, terragruntOptions)
	child := filepath.Dir(terragruntOptions.TerragruntConfigPath)
	return util.GetPathRelativeTo(parent, child)
}

func getparentLocalConfigFilesLocation(include IncludeConfig, terragruntOptions *options.TerragruntOptions) string {
	cursor := &include
	for {
		if cursor.Source == "" {
			return filepath.Dir(cursor.Path)
		}
		cursor = cursor.IncludeBy
	}
}

// Return the AWS account id associated to the current set of credentials
func getAWSAccountID() (string, error) {
	session, err := session.NewSession()
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	identity, err := sts.New(session).GetCallerIdentity(nil)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return *identity.Account, nil
}

func saveVariables(parameter string, terragruntOptions *options.TerragruntOptions) (interface{}, error) {
	matches := HELPER_FUNCTION_SINGLE_STRING_PARAMETER_SYNTAX_REGEX.FindStringSubmatch(parameter)
	if len(matches) != 2 {
		return "", errors.WithStackTrace(InvalidSaveVariablesParameter(parameter))
	}

	terragruntOptions.AddDeferredSaveVariables(matches[1])
	return matches[1], nil
}

// Custom error types

type InvalidInterpolationSyntax string

func (err InvalidInterpolationSyntax) Error() string {
	return fmt.Sprintf("Invalid interpolation syntax. Expected syntax of the form '${function_name()}', but got '%s'", string(err))
}

type UnknownHelperFunction string

func (err UnknownHelperFunction) Error() string {
	return fmt.Sprintf("Unknown helper function: %s", string(err))
}

type ParentTerragruntConfigNotFound string

func (err ParentTerragruntConfigNotFound) Error() string {
	return fmt.Sprintf("Could not find a Terragrunt config file in any of the parent folders of %s", string(err))
}

type CheckedTooManyParentFolders string

func (err CheckedTooManyParentFolders) Error() string {
	return fmt.Sprintf("Could not find a Terragrunt config file in a parent folder of %s after checking %d parent folders", string(err), MAX_PARENT_FOLDERS_TO_CHECK)
}

type InvalidFunctionParameters string

func (err InvalidFunctionParameters) Error() string {
	return fmt.Sprintf("Invalid parameters. Expected syntax of the form '${get_env(\"env\", \"default\")}', but got '%s'", string(err))
}

type InvalidSaveVariablesParameter string

func (err InvalidSaveVariablesParameter) Error() string {
	return fmt.Sprintf("Invalid parameters. Expected syntax of the form '${save_variables(\"filename.json\")}', but got '%s'", string(err))
}
