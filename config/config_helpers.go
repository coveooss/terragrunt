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

var INTERPOLATION_VARS = `\s*var\.([[:alpha:]][\w-]*)\s*`
var INTERPOLATION_PARAMETERS = `(\s*("[^"]*?"|var\.\w+)\s*,?\s*)*`
var INTERPOLATION_SYNTAX_REGEX = regexp.MustCompile(fmt.Sprintf(`\$\{\s*(\w+\(%s\)|%s)\s*\}`, INTERPOLATION_PARAMETERS, INTERPOLATION_VARS))
var INTERPOLATION_SYNTAX_REGEX_SINGLE = regexp.MustCompile(fmt.Sprintf(`"(%s)"`, INTERPOLATION_SYNTAX_REGEX))
var INTERPOLATION_SYNTAX_REGEX_REMAINING = regexp.MustCompile(`\$\{.*?\}`)
var HELPER_FUNCTION_SYNTAX_REGEX = regexp.MustCompile(`^\$\{(.*?)\((.*?)\)\}$`)
var HELPER_VAR_REGEX = regexp.MustCompile(fmt.Sprintf(`\$\{%s\}`, INTERPOLATION_VARS))
var HELPER_FUNCTION_GET_ENV_PARAMETERS_SYNTAX_REGEX = regexp.MustCompile(`^\s*"(?P<env>[^=]+?)"\s*\,` + getVarParams(1) + `$`)
var HELPER_FUNCTION_GET_DEFAULT_PARAMETERS_SYNTAX_REGEX = regexp.MustCompile(`^` + getVarParams(2) + `$`)
var HELPER_FUNCTION_GET_DISCOVER_PARAMETERS_SYNTAX_REGEX = regexp.MustCompile(`^\s*"(?P<tag>[^=]+?)"\s*\,` + getVarParams(2) + `$`)
var HELPER_FUNCTION_SINGLE_STRING_PARAMETER_SYNTAX_REGEX = regexp.MustCompile(`^\s*"(.*?)"\s*$`)
var MAX_PARENT_FOLDERS_TO_CHECK = 100

func getVarParams(count int) string {
	const parameterRegexBase = `\s*(?:"(?P<string%d>.*?)"|var\.(?P<var%d>[[:alpha:]][\w-]*)|(?P<func%d>\w+\(.*?\)))\s*`
	var params []string
	for i := 1; i <= count; i++ {
		params = append(params, fmt.Sprintf(parameterRegexBase, i, i, i))
	}
	return strings.Join(params, ",")
}

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

// List of terraform commands that accept -input=
var TERRAFORM_COMMANDS_NEED_INPUT = []string{
	"apply",
	"import",
	"init",
	"plan",
	"refresh",
}

type EnvVar struct {
	Name         string
	DefaultValue string
}

type resolveContext struct {
	include    IncludeConfig
	options    *options.TerragruntOptions
	parameters string
}

// Given a string value from a Terragrunt configuration, parse the string, resolve any calls to helper functions using
// the syntax ${...}, and return the final value.
func ResolveTerragruntConfigString(terragruntConfigString string, include IncludeConfig, terragruntOptions *options.TerragruntOptions) (string, error) {
	context := &resolveContext{include, terragruntOptions, ""}

	// First, we replace all single interpolation syntax (i.e. function directly enclosed within quotes "${function()}")
	terragruntConfigString, err := context.processSingleInterpolationInString(terragruntConfigString)
	if err != nil {
		return terragruntConfigString, err
	}
	// Then, we replace all other interpolation functions (i.e. functions not directly enclosed within quotes)
	return context.processMultipleInterpolationsInString(terragruntConfigString)
}

// Execute a single Terragrunt helper function and return its value as a string
func (context *resolveContext) executeTerragruntHelperFunction(functionName string, parameters string) (interface{}, error) {
	if functionMap == nil {
		// We only initialize the function mapping on the first call
		functionMap = map[string]interface{}{
			"find_in_parent_folders":                   (*resolveContext).findInParentFolders,
			"path_relative_to_include":                 (*resolveContext).pathRelativeToInclude,
			"path_relative_from_include":               (*resolveContext).pathRelativeFromInclude,
			"get_env":                                  (*resolveContext).getEnvironmentVariable,
			"default":                                  (*resolveContext).getDefaultValue,
			"discover":                                 (*resolveContext).getDiscoveredValue,
			"get_current_dir":                          (*resolveContext).getCurrentDir,
			"get_leaf_dir":                             (*resolveContext).getTfVarsDir,
			"get_tfvars_dir":                           (*resolveContext).getTfVarsDir,
			"get_parent_dir":                           (*resolveContext).getParentTfVarsDir,
			"get_parent_tfvars_dir":                    (*resolveContext).getParentTfVarsDir,
			"get_aws_account_id":                       (*resolveContext).getAWSAccountID,
			"save_variables":                           (*resolveContext).saveVariables,
			"get_terraform_commands_that_need_vars":    TERRAFORM_COMMANDS_NEED_VARS,
			"get_terraform_commands_that_need_locking": TERRAFORM_COMMANDS_NEED_LOCKING,
			"get_terraform_commands_that_need_input":   TERRAFORM_COMMANDS_NEED_INPUT,
			"get_temp_folder":                          GET_TEMP_FOLDER,
		}
	}

	// We create a new context with the parameters
	context = &resolveContext{context.include, context.options, parameters}
	switch invoke := functionMap[functionName].(type) {
	case func(*resolveContext) (interface{}, error):
		return invoke(context)
	case string:
		return invoke, nil
	case []string:
		return invoke, nil
	default:
		return "", errors.WithStackTrace(UnknownHelperFunction(functionName))
	}
}

var functionMap map[string]interface{}

// For all interpolation functions that are called using the syntax "${function_name()}" (i.e. single interpolation function within string,
// functions that return a non-string value we have to get rid of the surrounding quotes and convert the output to HCL syntax. For example,
// for an array, we need to return "v1", "v2", "v3".
func (context *resolveContext) processSingleInterpolationInString(terragruntConfigString string) (resolved string, finalErr error) {
	// The function we pass to ReplaceAllStringFunc cannot return an error, so we have to use named error parameters to capture such errors.
	resolved = INTERPOLATION_SYNTAX_REGEX_SINGLE.ReplaceAllStringFunc(terragruntConfigString, func(str string) string {
		fmt.Println("String to replace", str)
		matches := INTERPOLATION_SYNTAX_REGEX_SINGLE.FindStringSubmatch(str)

		out, err := context.resolveTerragruntInterpolation(matches[1])
		if err != nil {
			finalErr = err
			return str
		}

		switch out := out.(type) {
		case string:
			return fmt.Sprintf(`"%s"`, out)
		case []string:
			return util.CommaSeparatedStrings(out)
		default:
			return fmt.Sprintf("%v", out)
		}
	})
	return
}

// For all interpolation functions that are called using the syntax "${function_a()}-${function_b()}" (i.e. multiple interpolation function
// within the same string) or "Some text ${function_name()}" (i.e. string composition), we just replace the interpolation function call
// by the string representation of its return.
func (context *resolveContext) processMultipleInterpolationsInString(terragruntConfigString string) (resolved string, finalErr error) {
	// The function we pass to ReplaceAllStringFunc cannot return an error, so we have to use named error parameters to capture such errors.
	resolved = INTERPOLATION_SYNTAX_REGEX.ReplaceAllStringFunc(terragruntConfigString, func(str string) string {
		out, err := context.resolveTerragruntInterpolation(str)
		if err != nil {
			finalErr = err
			return str
		}

		return fmt.Sprintf("%v", out)
	})

	if finalErr == nil {
		// If there is no error, we check if there are remaining look-a-like interpolation strings
		// that have not been considered. If so, they are certainly malformed.
		remaining := INTERPOLATION_SYNTAX_REGEX_REMAINING.FindAllString(resolved, -1)
		if len(remaining) > 0 && context.options.EraseNonDefinedVariables {
			remaining = util.RemoveDuplicatesFromListKeepFirst(remaining)
			finalErr = InvalidInterpolationSyntax(strings.Join(remaining, ", "))
		}
	}

	return
}

// Substitute any variables in the string if there is a value associated with the variable
func (context *resolveContext) SubstituteVars(str string, terragruntOptions *options.TerragruntOptions) string {
	if newStr, ok := context.resolveTerragruntVars(str); ok {
		return newStr
	}
	return str
}

// Resolve the references to variables ${var.name} if there are
func (context *resolveContext) resolveTerragruntVars(str string) (string, bool) {
	var match = false
	str = HELPER_VAR_REGEX.ReplaceAllStringFunc(str, func(str string) string {
		match = true
		matches := HELPER_VAR_REGEX.FindStringSubmatch(str)
		if found, ok := context.options.Variables[matches[1]]; ok {
			return fmt.Sprint(found.Value)
		}
		if context.options.EraseNonDefinedVariables {
			if !warningDone[matches[0]] {
				context.options.Logger.Warningf("Variable %s undefined", matches[0])
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
func (context *resolveContext) resolveTerragruntInterpolation(str string) (interface{}, error) {
	if result, ok := context.resolveTerragruntVars(str); ok {
		return result, nil
	}

	matches := HELPER_FUNCTION_SYNTAX_REGEX.FindStringSubmatch(str)
	if len(matches) == 3 {
		return context.executeTerragruntHelperFunction(matches[1], matches[2])
	}

	return "", errors.WithStackTrace(InvalidInterpolationSyntax(str))
}

// Return the directory of the current include file that is processed
func (context *resolveContext) getCurrentDir() (interface{}, error) {
	return filepath.ToSlash(filepath.Dir(context.include.Path)), nil
}

// Return the directory where the Terragrunt configuration file lives
func (context *resolveContext) getTfVarsDir() (interface{}, error) {
	terragruntConfigFileAbsPath, err := filepath.Abs(context.options.TerragruntConfigPath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return filepath.ToSlash(filepath.Dir(terragruntConfigFileAbsPath)), nil
}

// Return the parent directory where the Terragrunt configuration file lives
func (context *resolveContext) getParentTfVarsDir() (interface{}, error) {
	parentPath, err := context.pathRelativeFromInclude()
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	currentPath := filepath.Dir(context.options.TerragruntConfigPath)
	parentPath, err = filepath.Abs(filepath.Join(currentPath, parentPath.(string)))
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return filepath.ToSlash(parentPath.(string)), nil
}

func (context *resolveContext) parseGetEnvParameters() (EnvVar, error) {
	envVariable := EnvVar{}
	matches := HELPER_FUNCTION_GET_ENV_PARAMETERS_SYNTAX_REGEX.FindStringSubmatch(context.parameters)
	if len(matches) < 4 {
		return envVariable, errors.WithStackTrace(InvalidFunctionParameters(context.parameters))
	}

	for index, name := range HELPER_FUNCTION_GET_ENV_PARAMETERS_SYNTAX_REGEX.SubexpNames() {
		value := strings.TrimSpace(matches[index])
		switch name {
		case "env":
			envVariable.Name = value
		case "string1":
			if value != "" {
				envVariable.DefaultValue = value
			}
		case "var1":
			if value != "" {
				varName := fmt.Sprintf("${var.%v}", value)
				envVariable.DefaultValue, _ = context.resolveTerragruntVars(varName)
			}
		}
	}

	return envVariable, nil
}

func (context *resolveContext) getEnvironmentVariable() (interface{}, error) {
	fmt.Printf("get_env(%s)\n", context.parameters)
	parameterMap, err := context.parseGetEnvParameters()

	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	envValue, exists := context.options.Env[parameterMap.Name]

	if !exists {
		envValue = parameterMap.DefaultValue
	}

	return envValue, nil
}

func (context *resolveContext) getDefaultValue() (interface{}, error) {
	parameters, err := context.getParameters(HELPER_FUNCTION_GET_DEFAULT_PARAMETERS_SYNTAX_REGEX)
	if err != nil {
		return "", fmt.Errorf(`Expecting default(var.name, "default")`)
	}

	if parameters[0] != "" {
		return parameters[0], nil
	}
	return parameters[1], nil
}

func (context *resolveContext) getParameters(regex *regexp.Regexp) ([]string, error) {
	matches := regex.FindStringSubmatch(context.parameters)
	if len(matches) != len(regex.SubexpNames()) {
		return nil, fmt.Errorf("Mistmatch number of parameters")
	}

	var result []string
	x := regexp.MustCompile(`^(string|var|func)(\d+)$`)
	for index, name := range regex.SubexpNames()[1:] {
		value := strings.TrimSpace(matches[index+1])

		subMatches := x.FindStringSubmatch(name)
		if len(subMatches) > 0 {
			switch subMatches[1] {
			case "string":
				result = append(result, value)
			case "var":
				i := len(result) - 1
				if value != "" {
					varName := fmt.Sprintf("${var.%v}", value)
					value, _ = context.resolveTerragruntVars(varName)
					if !strings.HasPrefix(value, "${") {
						result[i] = value
					}
				}
			case "func":
				i := len(result) - 1
				if value != "" {
					function := fmt.Sprintf("${%v}", value)
					funcResult, err := context.resolveTerragruntInterpolation(function)
					if err != nil {
						return nil, err
					}
					result[i] = fmt.Sprintf("%v", funcResult)
				}
			}
		} else {
			result = append(result, value)
		}
	}
	return result, nil
}

func (context *resolveContext) getDiscoveredValue() (interface{}, error) {
	matches := HELPER_FUNCTION_GET_DISCOVER_PARAMETERS_SYNTAX_REGEX.FindStringSubmatch(context.parameters)
	if len(matches) < 6 {
		err := fmt.Errorf(`Invalid parameters. Expected syntax of the form '${discover("tag", "key", "sault_key")}', but got '%s'`, context.parameters)
		return "", err
	}

	var tag, key string
	for index, name := range HELPER_FUNCTION_GET_DISCOVER_PARAMETERS_SYNTAX_REGEX.SubexpNames() {
		value := strings.TrimSpace(matches[index])
		switch name {
		case "tag":
			tag = value
		case "string1":
			if value != "" {
				key = value
			}
		case "var1":
			if value != "" {
				varName := fmt.Sprintf("${var.%v}", value)
				key, _ = context.resolveTerragruntVars(varName)
				if strings.HasPrefix(key, "${") {
					key = ""
				}
			}
		case "string2":
			if value != "" && key == "" {
				key = value
			}
		case "var2":
			if value != "" && key == "" {
				varName := fmt.Sprintf("${var.%v}", value)
				key, _ = context.resolveTerragruntVars(varName)
				if strings.HasPrefix(key, "${") {
					key = ""
				}
			}
		}
	}

	if key == "" && !context.options.EraseNonDefinedVariables {
		return "", nil
	}

	result, err := util.GetSSMParameter(fmt.Sprintf("%s/terragrunt/%s", key, tag))
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return result, nil
}

// Find a parent Terragrunt configuration file in the parent folders above the current Terragrunt configuration file
// and return its path
func (context *resolveContext) findInParentFolders() (interface{}, error) {
	previousDir, err := filepath.Abs(filepath.Dir(context.options.TerragruntConfigPath))
	previousDir = filepath.ToSlash(previousDir)

	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	// To avoid getting into an accidental infinite loop (e.g. do to cyclical symlinks), set a max on the number of
	// parent folders we'll check
	for i := 0; i < MAX_PARENT_FOLDERS_TO_CHECK; i++ {
		currentDir := filepath.ToSlash(filepath.Dir(previousDir))
		if currentDir == previousDir {
			return "", errors.WithStackTrace(ParentTerragruntConfigNotFound(context.options.TerragruntConfigPath))
		}

		configPath := DefaultConfigPath(currentDir)
		if util.FileExists(configPath) {
			return util.GetPathRelativeTo(configPath, filepath.Dir(context.options.TerragruntConfigPath))
		}

		previousDir = currentDir
	}

	return "", errors.WithStackTrace(CheckedTooManyParentFolders(context.options.TerragruntConfigPath))
}

// Return the relative path between the included Terragrunt configuration file and the current Terragrunt configuration
// file
func (context *resolveContext) pathRelativeToInclude() (interface{}, error) {
	parent := context.getParentLocalConfigFilesLocation()
	child := filepath.Dir(context.options.TerragruntConfigPath)
	return util.GetPathRelativeTo(child, parent)
}

// Return the relative path from the current Terragrunt configuration to the included Terragrunt configuration file
func (context *resolveContext) pathRelativeFromInclude() (interface{}, error) {
	parent := context.getParentLocalConfigFilesLocation()
	child := filepath.Dir(context.options.TerragruntConfigPath)
	return util.GetPathRelativeTo(parent, child)
}

func (context *resolveContext) getParentLocalConfigFilesLocation() string {
	cursor := &context.include
	for {
		if cursor.Source == "" {
			return filepath.Dir(cursor.Path)
		}
		cursor = cursor.IncludeBy
	}
}

// Return the AWS account id associated to the current set of credentials
func (context *resolveContext) getAWSAccountID() (interface{}, error) {
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

func (context *resolveContext) saveVariables() (interface{}, error) {
	matches := HELPER_FUNCTION_SINGLE_STRING_PARAMETER_SYNTAX_REGEX.FindStringSubmatch(context.parameters)
	if len(matches) != 2 {
		return "", errors.WithStackTrace(InvalidSaveVariablesParameter(context.parameters))
	}

	context.options.AddDeferredSaveVariables(matches[1])
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
