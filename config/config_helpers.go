package config

import (
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/coveooss/gotemplate/v3/collections"
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

type helperFunction struct {
	function   interface{}
	returnType cty.Type
}

func (context *resolveContext) getHelperFunctions() map[string]helperFunction {
	return map[string]helperFunction{
		"find_in_parent_folders":     {function: context.findInParentFolders},
		"path_relative_to_include":   {function: context.pathRelativeToInclude},
		"path_relative_from_include": {function: context.pathRelativeFromInclude},
		"get_env":                    {function: context.getEnvironmentVariable},
		"get_current_dir":            {function: context.getCurrentDir},
		"get_leaf_dir":               {function: context.getLeafDir},
		"get_tfvars_dir":             {function: context.getLeafDir},
		"get_parent_dir":             {function: context.getParentDir},
		"get_parent_tfvars_dir":      {function: context.getParentDir},
		"get_aws_account_id":         {function: context.getAWSAccountID},
		"set_global_variable":        {function: context.setGlobalVariable},
		"get_terraform_commands_that_need_vars": {
			function:   func() (interface{}, error) { return TerraformCommandWithVarFile, nil },
			returnType: cty.List(cty.String),
		},
		"get_terraform_commands_that_need_locking": {
			function:   func() (interface{}, error) { return TerraformCommandWithLockTimeout, nil },
			returnType: cty.List(cty.String),
		},
		"get_terraform_commands_that_need_input": {
			function:   func() (interface{}, error) { return TerraformCommandWithInput, nil },
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

	tfScope := tflang.Scope{
		BaseDir: filepath.Dir(context.include.Path),
	}
	for k, v := range tfScope.Functions() {
		functions[k] = v
	}

	for key, helperFunction := range context.getHelperFunctions() {
		key, helperFunction := key, helperFunction
		returnType := cty.String
		if helperFunction.returnType != cty.NilType {
			returnType = helperFunction.returnType
		}

		switch helperFunction.function.(type) {
		case func(string, interface{}) string:
			continue // Function receiving interface{} as argument are simply ignored
		case func() interface{}:
		case func() (interface{}, error):
		case func(string, string) interface{}:
		default:
			return nil, fmt.Errorf("unsupported function type %v for %s", reflect.TypeOf(helperFunction.function), key)
		}

		assert := func(test bool, format interface{}, args ...interface{}) {
			if !test {
				panic(fmt.Errorf(fmt.Sprint(format), args))
			}
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
			Impl: func(args []cty.Value, retType cty.Type) (result cty.Value, err error) {
				defer func() {
					if catched := recover(); catched != nil {
						err = catched.(error)
					}
				}()

				result = cty.NullVal(helperFunction.returnType)
				var out interface{}
				switch f := helperFunction.function.(type) {
				case func() interface{}:
					assert(len(args) == 0, "call to function %s should not have arguments", key)
					out = f()
				case func() (interface{}, error):
					assert(len(args) == 0, "call to function %s should not have arguments", key)
					out, err = f()
				case func(string, string) interface{}:
					assert(len(args) == 2, "call to function %s must have two arguments", key)
					out = f(args[0].AsString(), args[1].AsString())
				}

				assert(err == nil, err)
				if returnType == cty.String {
					return cty.StringVal(out.(string)), nil
				} else if returnType == cty.List(cty.String) {
					outVals := []cty.Value{}
					for _, val := range out.([]string) {
						outVals = append(outVals, cty.StringVal(val))
					}
					return cty.ListVal(outVals), nil
				}
				panic(fmt.Errorf("unsupported return type to %s. Type: %s", key, returnType))
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
func (context *resolveContext) getCurrentDir() interface{} {
	return filepath.ToSlash(filepath.Dir(context.include.Path))
}

// Return the directory where the Terragrunt configuration file lives
func (context *resolveContext) getLeafDir() (interface{}, error) {
	terragruntConfigFileAbsPath, err := filepath.Abs(context.options.TerragruntConfigPath)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Dir(terragruntConfigFileAbsPath)), nil
}

// Return the parent directory where the Terragrunt configuration file lives
func (context *resolveContext) getParentDir() (interface{}, error) {
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
func (context *resolveContext) getEnvironmentVariable(env, defValue string) interface{} {
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
func (context *resolveContext) findInParentFolders() (interface{}, error) {
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
func (context *resolveContext) getAWSAccountID() (interface{}, error) {
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

func (context *resolveContext) setGlobalVariable(key string, value interface{}) string {
	if key == "" {
		for key, value := range collections.AsDictionary(value).AsMap() {
			context.options.SetVariable(key, value, options.FunctionOverwrite)
		}
	} else {
		context.options.SetVariable(key, value, options.FunctionOverwrite)
	}
	return ""
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
