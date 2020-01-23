package config

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gruntwork-io/terragrunt/awshelper"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	tflang "github.com/hashicorp/terraform/lang"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var maxParentFoldersToCheck = 100

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

type resolveContext struct {
	include IncludeConfig
	options *options.TerragruntOptions
}

func (context *resolveContext) ErrorOnUndefined() bool {
	return !context.options.IgnoreRemainingInterpolation
}

type helperFunction struct {
	function   func(parameters ...string) (interface{}, error)
	returnType cty.Type
}

func (context *resolveContext) getHelperFunctions() map[string]helperFunction {
	return map[string]helperFunction{
		"find_in_parent_folders":     helperFunction{function: context.findInParentFolders},
		"path_relative_to_include":   helperFunction{function: context.pathRelativeToInclude},
		"path_relative_from_include": helperFunction{function: context.pathRelativeFromInclude},
		"get_env":                    helperFunction{function: context.getEnvironmentVariable},
		"get_current_dir":            helperFunction{function: context.getCurrentDir},
		"get_leaf_dir":               helperFunction{function: context.getLeafDir},
		"get_tfvars_dir":             helperFunction{function: context.getLeafDir},
		"get_parent_dir":             helperFunction{function: context.getParentDir},
		"get_parent_tfvars_dir":      helperFunction{function: context.getParentDir},
		"get_aws_account_id":         helperFunction{function: context.getAWSAccountID},
		"get_terraform_commands_that_need_vars": helperFunction{
			function:   func(...string) (interface{}, error) { return TerraformCommandWithVarFile, nil },
			returnType: cty.List(cty.String),
		},
		"get_terraform_commands_that_need_locking": helperFunction{
			function:   func(...string) (interface{}, error) { return TerraformCommandWithLockTimeout, nil },
			returnType: cty.List(cty.String),
		},
		"get_terraform_commands_that_need_input": helperFunction{
			function:   func(...string) (interface{}, error) { return TerraformCommandWithInput, nil },
			returnType: cty.List(cty.String),
		},
	}
}

func (context *resolveContext) getHelperFunctionsInterfaces() map[string]interface{} {
	functions := map[string]interface{}{}
	for key, function := range context.getHelperFunctions() {
		functions[key] = function.function
	}
	return functions
}

// Create an EvalContext for the HCL2 parser.
// We can define functions and variables in this context that the HCL2 parser will make available to the Terragrunt configuration during parsing.
func (context *resolveContext) getHelperFunctionsHCLContext() (*hcl.EvalContext, error) {
	functions := map[string]function.Function{}

	tfscope := tflang.Scope{
		BaseDir: filepath.Dir(context.include.Path),
	}
	for k, v := range tfscope.Functions() {
		functions[k] = v
	}

	for key, helperFunction := range context.getHelperFunctions() {
		helperFunction := helperFunction
		returnType := cty.String
		if helperFunction.returnType != cty.NilType {
			returnType = helperFunction.returnType
		}
		functions[key] = function.New(&function.Spec{
			Type: function.StaticReturnType(returnType),
			VarParam: &function.Parameter{
				Name:             "vals",
				Type:             cty.DynamicPseudoType,
				AllowUnknown:     true,
				AllowDynamicType: true,
				AllowNull:        true,
			},
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				argStrings, err := ctySliceToStringSlice(args)
				if err != nil {
					return cty.NullVal(helperFunction.returnType), err
				}
				out, err := helperFunction.function(argStrings...)
				if err != nil {
					return cty.NullVal(helperFunction.returnType), err
				}
				if returnType == cty.String {
					return cty.StringVal(out.(string)), nil
				} else if returnType == cty.List(cty.String) {
					outVals := []cty.Value{}
					for _, val := range out.([]string) {
						outVals = append(outVals, cty.StringVal(val))
					}
					return cty.ListVal(outVals), nil
				}
				return cty.NullVal(helperFunction.returnType), fmt.Errorf("unsupported return type to %s. Type: %s", key, returnType)
			},
		})
	}

	variables := context.options.GetContext()
	// Legacy, variables used to be called with `var.`
	variables.Set("var", variables.Clone())

	ctyVariables, err := util.ToCtyValue(variables.AsMap())
	if err != nil {
		return nil, err
	}
	return &hcl.EvalContext{Functions: functions, Variables: ctyVariables.AsValueMap()}, nil
}

// Return the directory of the current include file that is processed
func (context *resolveContext) getCurrentDir(...string) (interface{}, error) {
	return filepath.ToSlash(filepath.Dir(context.include.Path)), nil
}

// Return the directory where the Terragrunt configuration file lives
func (context *resolveContext) getLeafDir(...string) (interface{}, error) {
	terragruntConfigFileAbsPath, err := filepath.Abs(context.options.TerragruntConfigPath)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Dir(terragruntConfigFileAbsPath)), nil
}

// Return the parent directory where the Terragrunt configuration file lives
func (context *resolveContext) getParentDir(...string) (interface{}, error) {
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

// Returns the named environment variable or default value if it does not exist
//     get_env(variable_name, default_value)
func (context *resolveContext) getEnvironmentVariable(parameters ...string) (interface{}, error) {
	if parameters[0] == "" {
		return "", invalidGetEnvParameters(parameters)
	}
	return context.getEnvironmentVariableInternal(parameters[0], parameters[1]), nil
}

func (context *resolveContext) getEnvironmentVariableInternal(env, defValue string) interface{} {
	if value, exists := context.options.Env[env]; exists {
		return value
	}
	return defValue
}

type invalidGetEnvParameters []string

func (err invalidGetEnvParameters) Error() string {
	return fmt.Sprintf("Invalid parameters. Expected get_env(variable_name, default_value) but got '%s'", strings.Join(err, ", "))
}

// Find a parent Terragrunt configuration file in the parent folders above the current Terragrunt configuration file
// and return its path
func (context *resolveContext) findInParentFolders(...string) (interface{}, error) {
	previousDir, err := filepath.Abs(filepath.Dir(context.options.TerragruntConfigPath))
	previousDir = filepath.ToSlash(previousDir)

	if err != nil {
		return "", err
	}

	// To avoid getting into an accidental infinite loop (e.g. do to cyclical symlinks), set a max on the number of
	// parent folders we'll check
	for i := 0; i < maxParentFoldersToCheck; i++ {
		currentDir := filepath.ToSlash(filepath.Dir(previousDir))
		if currentDir == previousDir {
			return "", parentTerragruntConfigNotFound(context.options.TerragruntConfigPath)
		}

		configPath := util.JoinPath(currentDir, DefaultTerragruntConfigPath)
		if util.FileExists(configPath) {
			return util.GetPathRelativeTo(configPath, filepath.Dir(context.options.TerragruntConfigPath))
		}

		previousDir = currentDir
	}

	return "", checkedTooManyParentFolders(context.options.TerragruntConfigPath)
}

type parentTerragruntConfigNotFound string

func (err parentTerragruntConfigNotFound) Error() string {
	return fmt.Sprintf("Could not find a Terragrunt config file in any of the parent folders of %s", string(err))
}

type checkedTooManyParentFolders string

func (err checkedTooManyParentFolders) Error() string {
	return fmt.Sprintf("Could not find a Terragrunt config file in a parent folder of %s after checking %d parent folders", string(err), maxParentFoldersToCheck)
}

// Return the relative path between the included Terragrunt configuration file and the current Terragrunt configuration
// file
func (context *resolveContext) pathRelativeToInclude(...string) (interface{}, error) {
	parent := context.getParentLocalConfigFilesLocation()
	child := filepath.Dir(context.options.TerragruntConfigPath)
	return util.GetPathRelativeTo(child, parent)
}

// Return the relative path from the current Terragrunt configuration to the included Terragrunt configuration file
func (context *resolveContext) pathRelativeFromInclude(...string) (interface{}, error) {
	parent := context.getParentLocalConfigFilesLocation()
	child := filepath.Dir(context.options.TerragruntConfigPath)
	return util.GetPathRelativeTo(parent, child)
}

func (context *resolveContext) getParentLocalConfigFilesLocation() string {
	for cursor := &context.include; cursor != nil; cursor = cursor.isIncludedBy {
		includePath := cursor.Path
		if !cursor.isBootstrap {
			if !path.IsAbs(includePath) {
				includePath = util.JoinPath(context.options.WorkingDir, includePath)
			}
			return filepath.Dir(includePath)
		}
	}
	return ""
}

// Return the AWS account id associated to the current set of credentials
func (context *resolveContext) getAWSAccountID(...string) (interface{}, error) {
	session, err := awshelper.CreateAwsSession("", "")
	if err != nil {
		return "", err
	}

	identity, err := sts.New(session).GetCallerIdentity(nil)
	if err != nil {
		return "", err
	}

	return *identity.Account, nil
}

// Convert the slice of cty values to a slice of strings. If any of the values in the given slice is not a string,
// return an error.
func ctySliceToStringSlice(args []cty.Value) ([]string, error) {
	var out []string
	for _, arg := range args {
		if arg.Type() != cty.String {
			return nil, errors.WithStackTrace(InvalidParameterType{Expected: "string", Actual: arg.Type().FriendlyName()})
		}
		out = append(out, arg.AsString())
	}
	return out, nil
}

type InvalidParameterType struct {
	Expected string
	Actual   string
}

func (err InvalidParameterType) Error() string {
	return fmt.Sprintf("Expected param of type %s but got %s", err.Expected, err.Actual)
}
