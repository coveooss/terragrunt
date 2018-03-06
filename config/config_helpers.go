package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

const (
	GET_TEMP_FOLDER   = "<TEMP_FOLDER>"
	GET_SCRIPT_FOLDER = "<SCRIPT_FOLDER>"
)

var (
	INTERPOLATION_VARS                   = `var\.([\p{L}_][\p{L}_\-\d\.]*)\s*`
	INTERPOLATION_PARAMETERS             = fmt.Sprintf(`(\s*(%s)\s*,?\s*)*`, getVarParams(1))
	INTERPOLATION_SYNTAX_REGEX           = regexp.MustCompile(fmt.Sprintf(`\$\{\s*(\w+\(%s\)|%s)\s*\}`, INTERPOLATION_PARAMETERS, INTERPOLATION_VARS))
	INTERPOLATION_SYNTAX_REGEX_SINGLE    = regexp.MustCompile(fmt.Sprintf(`"(%s)"`, INTERPOLATION_SYNTAX_REGEX))
	INTERPOLATION_SYNTAX_REGEX_REMAINING = regexp.MustCompile(`\$\{.*?\}`)
	HELPER_FUNCTION_SYNTAX_REGEX         = regexp.MustCompile(`^\$\{\s*(.*?)\((.*?)\)\s*\}$`)
	HELPER_VAR_REGEX                     = regexp.MustCompile(fmt.Sprintf(`\$\{%s\}`, INTERPOLATION_VARS))
	MAX_PARENT_FOLDERS_TO_CHECK          = 100
)

func getVarParams(count int) string {
	const parameterRegexBase = `\s*(?:"(?P<string%d>[^\"]*?)"|var\.(?P<var%d>[[:alpha:]][\w-]*)|(?P<func%d>\w+\(.*?\)))\s*`
	var params []string
	for i := 1; i <= count; i++ {
		params = append(params, fmt.Sprintf(parameterRegexBase, i, i, i))
	}
	return strings.Join(params, ",")
}

/*
To help identifying terraform commands that requires specific args, you can use the following function in a bash shell:

function find-tf-usage() {
	local arg
	for arg in "$@"
	do
		echo
		local name="TerraformCommandWith$(echo ${arg} | sed -r 's/(^|[-_])([a-z])/\U\2/g')"
		echo "// $name is the list of Terraform commands accepting -${arg}"
		echo "var $name = []string{"
		terraform |
			sed -E "s/^\s{4}/CMD /g" |
			grep "^CMD " |
			cut -f2 -d' ' |
			xargs -n1 -I{} sh -c 'f() { terraform {} --help 2>&1| grep -- "${1}[ =]" >/dev/null&& echo "    \"{}\",";}; f $1' - $arg
		echo "}"
	done
}

find-tf-usage -lock-timeout var-file -input
*/

// TerraformCommandWithLockTimeout is the list of Terraform commands accepting --lock-timeout
var TerraformCommandWithLockTimeout = []string{
	"apply",
	"destroy",
	"import",
	"init",
	"plan",
	"refresh",
	"taint",
	"untaint",
}

// TerraformCommandWithVarFile is the list of Terraform commands accepting -var-file
var TerraformCommandWithVarFile = []string{
	"apply",
	"console",
	"destroy",
	"import",
	"plan",
	"push",
	"refresh",
	"validate",
}

// TerraformCommandWithInput is the list of Terraform commands accepting --input
var TerraformCommandWithInput = []string{
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

func (context *resolveContext) ErrorOnUndefined() bool {
	return !context.options.IgnoreRemainingInterpolation
}

// Given a string value from a Terragrunt configuration, parse the string, resolve any calls to helper functions using
// the syntax ${...}, and return the final value.
func ResolveTerragruntConfigString(terragruntConfigString string, include IncludeConfig, terragruntOptions *options.TerragruntOptions) (string, error) {
	context := &resolveContext{
		include: include,
		options: terragruntOptions,
	}

	// First, we replace all single interpolation syntax (i.e. function directly enclosed within quotes "${function()}")
	terragruntConfigString, err := context.processSingleInterpolationInString(terragruntConfigString)
	if err != nil {
		return terragruntConfigString, err
	}
	// Then, we replace all other interpolation functions (i.e. functions not directly enclosed within quotes)
	return context.processMultipleInterpolationsInString(terragruntConfigString)
}

// SubstituteVars substitutes any variables in the string if there is a value associated with the variable
func SubstituteVars(str string, terragruntOptions *options.TerragruntOptions) string {
	context := &resolveContext{options: terragruntOptions}
	if newStr, ok := context.resolveTerragruntVars(str); ok {
		return newStr
	}
	return str
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
			"get_terraform_commands_that_need_vars":    TerraformCommandWithVarFile,
			"get_terraform_commands_that_need_locking": TerraformCommandWithLockTimeout,
			"get_terraform_commands_that_need_input":   TerraformCommandWithInput,
			"get_temp_folder":                          GET_TEMP_FOLDER,
			"get_script_folder":                        GET_SCRIPT_FOLDER,
		}
	}

	// We create a new context with the parameters
	context = &resolveContext{
		include:    context.include,
		options:    context.options,
		parameters: parameters,
	}
	switch invoke := functionMap[functionName].(type) {
	case func(*resolveContext) (interface{}, error):
		result, err := invoke(context)
		if err != nil {
			err = errors.WithStackTraceAndPrefix(err, "Error while calling %s(%s)", functionName, parameters)
		}
		return result, err
	case string, []string:
		return invoke, nil
	default:
		return "", errors.WithStackTrace(UnknownHelperFunction(functionName))
	}
}

var functionMap map[string]interface{}

type UnknownHelperFunction string

func (err UnknownHelperFunction) Error() string {
	return fmt.Sprintf("Unknown helper function: %s", string(err))
}

// For all interpolation functions that are called using the syntax "${function_name()}" (i.e. single interpolation function within string,
// functions that return a non-string value we have to get rid of the surrounding quotes and convert the output to HCL syntax. For example,
// for an array, we need to return "v1", "v2", "v3".
func (context *resolveContext) processSingleInterpolationInString(terragruntConfigString string) (resolved string, finalErr error) {
	// The function we pass to ReplaceAllStringFunc cannot return an error, so we have to use named error parameters to capture such errors.
	resolved = INTERPOLATION_SYNTAX_REGEX_SINGLE.ReplaceAllStringFunc(terragruntConfigString, func(str string) string {
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
		if len(remaining) > 0 {
			if resolved == terragruntConfigString && context.ErrorOnUndefined() {
				remaining = util.RemoveDuplicatesFromListKeepFirst(remaining)
				finalErr = InvalidInterpolationSyntax(strings.Join(remaining, ", "))
			} else if resolved != terragruntConfigString {
				// There was a change, so we retry the conversion to catch cases where there is an
				// interpolation in the interpolation "${func("${func2()}")}" which is legit in
				// terraform
				return context.processMultipleInterpolationsInString(resolved)
			}
		}
	}

	return
}

// Resolve the references to variables ${var.name} if there are
func (context *resolveContext) resolveTerragruntVars(str string) (string, bool) {
	var match = false
	str = HELPER_VAR_REGEX.ReplaceAllStringFunc(str, func(str string) string {
		match = true
		matches := HELPER_VAR_REGEX.FindStringSubmatch(str)
		if strings.Contains(matches[1], ".") {
			if result, ok := context.resolveTerragruntMapVars(matches[1]); ok {
				return result
			}
		} else if found, ok := context.options.Variables[matches[1]]; ok {
			result := fmt.Sprint(found.Value)
			if strings.Contains(result, "#{") {
				delayedVar := strings.Replace(result, "#{", "${", 1)
				if resolvedValue, ok := context.resolveTerragruntVars(delayedVar); ok {
					context.options.SetVariable(matches[1], resolvedValue, found.Source)
					result = resolvedValue
				}
			}
			return result
		}

		if context.ErrorOnUndefined() {
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

// resolveTerragruntMapVars returns the value of map variable element, i.e. var.a.b
func (context *resolveContext) resolveTerragruntMapVars(str string) (string, bool) {
	selection := strings.Split(str, ".")
	if found, ok := context.options.Variables[selection[0]]; ok {
		v := found.Value
		for i, sel := range selection[1:] {
			if reflect.TypeOf(v).Kind() != reflect.Map {
				name := strings.Join(selection[:i+1], ".")
				if !warningDone[name] {
					context.options.Logger.Errorf("Variable %s must be a map", name)
					warningDone[name] = true
				}
				return "", false
			}
			element := reflect.ValueOf(v).MapIndex(reflect.ValueOf(sel))
			if !element.IsValid() {
				return "", false
			}
			v = element.Interface()
		}
		return fmt.Sprint(v), true
	}
	return "", false
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

type InvalidInterpolationSyntax string

func (err InvalidInterpolationSyntax) Error() string {
	return fmt.Sprintf("Invalid interpolation syntax. Expected syntax of the form '${function_name()}', but got '%s'", string(err))
}

// Return the directory of the current include file that is processed
func (context *resolveContext) getCurrentDir() (interface{}, error) {
	return filepath.ToSlash(filepath.Dir(context.include.Path)), nil
}

// Return the directory where the Terragrunt configuration file lives
func (context *resolveContext) getTfVarsDir() (interface{}, error) {
	terragruntConfigFileAbsPath, err := filepath.Abs(context.options.TerragruntConfigPath)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Dir(terragruntConfigFileAbsPath)), nil
}

// Return the parent directory where the Terragrunt configuration file lives
func (context *resolveContext) getParentTfVarsDir() (interface{}, error) {
	parentPath, err := context.pathRelativeFromInclude()
	if err != nil {
		return "", err
	}

	currentPath := filepath.Dir(context.options.TerragruntConfigPath)
	parentPath, err = filepath.Abs(filepath.Join(currentPath, parentPath.(string)))
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(parentPath.(string)), nil
}

var p1Regex = regexp.MustCompile(`^` + getVarParams(1) + `$`)
var p2Regex = regexp.MustCompile(`^` + getVarParams(2) + `$`)
var p3Regex = regexp.MustCompile(`^` + getVarParams(3) + `$`)

// Returns the named environment variable or default value if it does not exist
//     get_env(variable_name, default_value)
func (context *resolveContext) getEnvironmentVariable() (interface{}, error) {
	parameters, err := context.getParameters(p2Regex)
	if err != nil || parameters[0] == "" {
		return "", InvalidGetEnvParameters(context.parameters)
	}
	if value, exists := context.options.Env[parameters[0]]; exists {
		return value, nil
	}

	return parameters[1], nil
}

type InvalidGetEnvParameters string

func (err InvalidGetEnvParameters) Error() string {
	return fmt.Sprintf("Invalid parameters. Expected get_env(variable_name, default_value) but got '%s'", string(err))
}

// Returns the value of a variable or default value if the variable is not defined
//     default(var.name, default_value)
func (context *resolveContext) getDefaultValue() (interface{}, error) {
	parameters, err := context.getParameters(p2Regex)
	if err != nil {
		return "", InvalidDefaultParameters(context.parameters)
	}

	if strings.HasPrefix(parameters[0], "${") {
		return parameters[1], nil
	}
	return parameters[0], nil
}

type InvalidDefaultParameters string

func (err InvalidDefaultParameters) Error() string {
	return fmt.Sprintf("Invalid parameters. Expected default(var.name, default) but got '%s'", string(err))
}

// Returns the value from the parameter store
//     discover(key_name, folder, region)
func (context *resolveContext) getDiscoveredValue() (interface{}, error) {
	parameters, err := context.getParameters(p3Regex)
	if err != nil {
		return "", InvalidDiscoveryParameters(context.parameters)
	}

	result, err := aws_helper.GetSSMParameter(fmt.Sprintf("/%s/terragrunt/%s", parameters[1], parameters[0]), parameters[2])
	if err != nil {
		return "", ErrorOnDiscovery{err, context.parameters}
	}

	return result, nil
}

type InvalidDiscoveryParameters string

func (err InvalidDiscoveryParameters) Error() string {
	return fmt.Sprintf("Invalid parameters. Expected discover(key, env, region) but got '%s'", string(err))
}

type ErrorOnDiscovery struct {
	sourceError error
	parameters  string
}

func (err ErrorOnDiscovery) Error() string {
	return fmt.Sprintf("Error while calling discovery(%s): %v", err.parameters, err.sourceError)
}

// Saves variables into a file
//     save_variables(filename)
func (context *resolveContext) saveVariables() (interface{}, error) {
	parameters, err := context.getParameters(p1Regex)
	if err != nil {
		return "", InvalidSaveVariablesParameters(context.parameters)
	}

	context.options.AddDeferredSaveVariables(parameters[0])
	return parameters[0], nil
}

type InvalidSaveVariablesParameters string

func (err InvalidSaveVariablesParameters) Error() string {
	return fmt.Sprintf("Invalid parameters. Expected save_variables(filename) but got '%s'", string(err))
}

// Find a parent Terragrunt configuration file in the parent folders above the current Terragrunt configuration file
// and return its path
func (context *resolveContext) findInParentFolders() (interface{}, error) {
	previousDir, err := filepath.Abs(filepath.Dir(context.options.TerragruntConfigPath))
	previousDir = filepath.ToSlash(previousDir)

	if err != nil {
		return "", err
	}

	// To avoid getting into an accidental infinite loop (e.g. do to cyclical symlinks), set a max on the number of
	// parent folders we'll check
	for i := 0; i < MAX_PARENT_FOLDERS_TO_CHECK; i++ {
		currentDir := filepath.ToSlash(filepath.Dir(previousDir))
		if currentDir == previousDir {
			return "", ParentTerragruntConfigNotFound(context.options.TerragruntConfigPath)
		}

		configPath := DefaultConfigPath(currentDir)
		if util.FileExists(configPath) {
			return util.GetPathRelativeTo(configPath, filepath.Dir(context.options.TerragruntConfigPath))
		}

		previousDir = currentDir
	}

	return "", CheckedTooManyParentFolders(context.options.TerragruntConfigPath)
}

type ParentTerragruntConfigNotFound string

func (err ParentTerragruntConfigNotFound) Error() string {
	return fmt.Sprintf("Could not find a Terragrunt config file in any of the parent folders of %s", string(err))
}

type CheckedTooManyParentFolders string

func (err CheckedTooManyParentFolders) Error() string {
	return fmt.Sprintf("Could not find a Terragrunt config file in a parent folder of %s after checking %d parent folders", string(err), MAX_PARENT_FOLDERS_TO_CHECK)
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
	session, err := aws_helper.CreateAwsSession("", "")
	if err != nil {
		return "", err
	}

	identity, err := sts.New(session).GetCallerIdentity(nil)
	if err != nil {
		return "", err
	}

	return *identity.Account, nil
}

func (context *resolveContext) getParameters(regex *regexp.Regexp) ([]string, error) {
	matches := regex.FindStringSubmatch(context.parameters)
	if len(matches) != len(regex.SubexpNames()) {
		return nil, fmt.Errorf("Mistmatch number of parameters")
	}

	var result []string
	for index, name := range regex.SubexpNames()[1:] {
		value := strings.TrimSpace(matches[index+1])

		subMatches := parameterTypeRegex.FindStringSubmatch(name)
		if len(subMatches) > 0 {
			switch subMatches[1] {
			case "string":
				result = append(result, value)
			case "var":
				i := len(result) - 1
				if value != "" {
					varName := fmt.Sprintf("${var.%v}", value)
					result[i], _ = context.resolveTerragruntVars(varName)
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

var parameterTypeRegex = regexp.MustCompile(`^(string|var|func)(\d+)$`)
