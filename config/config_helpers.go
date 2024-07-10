package config

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/terragrunt/v2/awshelper"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
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

func (ctx *resolveContext) getHelperFunctions() map[string]helperFunction {
	return map[string]helperFunction{
		"find_in_parent_folders":     {function: ctx.findInParentFolders},
		"path_relative_to_include":   {function: ctx.pathRelativeToInclude},
		"path_relative_from_include": {function: ctx.pathRelativeFromInclude},
		"get_env":                    {function: ctx.getEnvironmentVariable},
		"get_current_dir":            {function: ctx.getCurrentDir},
		"get_leaf_dir":               {function: ctx.getLeafDir},
		"get_tfvars_dir":             {function: ctx.getLeafDir},
		"get_parent_dir":             {function: ctx.getParentDir},
		"get_parent_tfvars_dir":      {function: ctx.getParentDir},
		"get_aws_account_id":         {function: ctx.getAWSAccountID},
		"set_global_variable":        {function: ctx.setGlobalVariable},
		"get_terraform_commands_that_need_vars": {
			function:   func() ([]string, error) { return TerraformCommandWithVarFile, nil },
			returnType: cty.List(cty.String),
		},
		"get_terraform_commands_that_need_locking": {
			function:   func() ([]string, error) { return TerraformCommandWithLockTimeout, nil },
			returnType: cty.List(cty.String),
		},
		"get_terraform_commands_that_need_input": {
			function:   func() ([]string, error) { return TerraformCommandWithInput, nil },
			returnType: cty.List(cty.String),
		},
	}
}

func (ctx *resolveContext) getHelperFunctionsInterfaces() map[string]interface{} {
	functions := map[string]interface{}{}
	for key, function := range ctx.getHelperFunctions() {
		functions[key] = function.function
	}
	return functions
}

// Create an EvalContext for the HCL2 parser.
// We can define functions and variables in this context that the HCL2 parser will make available to the Terragrunt configuration during parsing.
func (ctx *resolveContext) getHelperFunctionsHCLContext() (*hcl.EvalContext, error) {
	functions := map[string]function.Function{}

	tfScope := tflang.Scope{
		BaseDir: filepath.Dir(ctx.include.Path),
	}
	for k, v := range tfScope.Functions() {
		functions[k] = v
	}

	for key, helperFunction := range ctx.getHelperFunctions() {
		key, helperFunction := key, helperFunction
		returnType := cty.String
		if helperFunction.returnType != cty.NilType {
			returnType = helperFunction.returnType
		}

		switch helperFunction.function.(type) {
		case func(string, interface{}) string:
			continue // Function receiving interface{} as argument are simply ignored
		case func() string:
		case func() (string, error):
		case func() ([]string, error):
		case func(string, string) string:
		default:
			return nil, fmt.Errorf("unsupported function type %v for %s", reflect.TypeOf(helperFunction.function), key)
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
				defer tgerrors.Recover(func(cause error) { err = cause })

				result = cty.NullVal(helperFunction.returnType)
				var out interface{}
				switch f := helperFunction.function.(type) {
				case func() string:
					tgerrors.Assert(len(args) == 0, "call to function %s should not have arguments", key)
					out = f()
				case func() (string, error):
					tgerrors.Assert(len(args) == 0, "call to function %s should not have arguments", key)
					out, err = f()
				case func() ([]string, error):
					tgerrors.Assert(len(args) == 0, "call to function %s should not have arguments", key)
					out, err = f()
				case func(string, string) string:
					tgerrors.Assert(len(args) == 2, "call to function %s must have two arguments", key)
					out = f(args[0].AsString(), args[1].AsString())
				}
				tgerrors.Assert(err == nil, err)

				if returnType == cty.String {
					return cty.StringVal(out.(string)), nil
				} else if returnType == cty.List(cty.String) {
					outVals := []cty.Value{}
					for _, val := range out.([]string) {
						outVals = append(outVals, cty.StringVal(val))
					}
					return cty.ListVal(outVals), nil
				}
				return result, fmt.Errorf("unsupported return type to %s. Type: %s", key, returnType)
			},
		})
	}

	variables := ctx.options.GetContext()
	// Legacy, variables used to be called with `var.`
	variables.Set("var", variables.Clone())

	ctyVariables, err := util.ToCtyValue(variables.AsMap())
	if err != nil {
		return nil, err
	}
	return &hcl.EvalContext{Functions: functions, Variables: ctyVariables.AsValueMap()}, nil
}

// Return the directory of the current include file that is processed
func (ctx *resolveContext) getCurrentDir() string {
	return filepath.ToSlash(filepath.Dir(ctx.include.Path))
}

// Return the directory where the Terragrunt configuration file lives
func (ctx *resolveContext) getLeafDir() (string, error) {
	terragruntConfigFileAbsPath, err := filepath.Abs(ctx.options.TerragruntConfigPath)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Dir(terragruntConfigFileAbsPath)), nil
}

// Return the parent directory where the Terragrunt configuration file lives
func (ctx *resolveContext) getParentDir() (string, error) {
	parentPath, err := ctx.pathRelativeFromInclude()
	if err != nil {
		return "", err
	}

	currentPath := filepath.Dir(ctx.options.TerragruntConfigPath)
	parentPath, err = filepath.Abs(filepath.Join(currentPath, parentPath))
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(parentPath), nil
}

// Returns the named environment variable or default value if it does not exist
//
//	get_env(variable_name, default_value)
func (ctx *resolveContext) getEnvironmentVariable(env, defValue string) string {
	if value, exists := ctx.options.Env[env]; exists {
		return value
	}
	return defValue
}

// Find a parent Terragrunt configuration file in the parent folders above the current Terragrunt configuration file
// and return its path
func (ctx *resolveContext) findInParentFolders() (string, error) {
	previousDir, err := filepath.Abs(filepath.Dir(ctx.options.TerragruntConfigPath))
	previousDir = filepath.ToSlash(previousDir)

	if err != nil {
		return "", err
	}

	// To avoid getting into an accidental infinite loop (e.g. do to cyclical symlinks), set a max on the number of
	// parent folders we'll check
	for i := 0; i < maxParentFoldersToCheck; i++ {
		currentDir := filepath.ToSlash(filepath.Dir(previousDir))
		if currentDir == previousDir {
			return "", parentTerragruntConfigNotFound(ctx.options.TerragruntConfigPath)
		}

		if configPath, exists := ctx.options.ConfigPath(currentDir); exists {
			return util.GetPathRelativeTo(configPath, filepath.Dir(ctx.options.TerragruntConfigPath))
		}

		previousDir = currentDir
	}

	return "", checkedTooManyParentFolders(ctx.options.TerragruntConfigPath)
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
func (ctx *resolveContext) pathRelativeToInclude() (string, error) {
	parent := ctx.getParentLocalConfigFilesLocation()
	child := filepath.Dir(ctx.options.TerragruntConfigPath)
	return util.GetPathRelativeTo(child, parent)
}

// Return the relative path from the current Terragrunt configuration to the included Terragrunt configuration file
func (ctx *resolveContext) pathRelativeFromInclude() (string, error) {
	parent := ctx.getParentLocalConfigFilesLocation()
	child := filepath.Dir(ctx.options.TerragruntConfigPath)
	return util.GetPathRelativeTo(parent, child)
}

func (ctx *resolveContext) getParentLocalConfigFilesLocation() string {
	for cursor := &ctx.include; cursor != nil; cursor = cursor.isIncludedBy {
		includePath := cursor.Path
		if !cursor.isBootstrap {
			if !path.IsAbs(includePath) {
				includePath = util.JoinPath(ctx.options.WorkingDir, includePath)
			}
			return filepath.Dir(includePath)
		}
	}
	return ""
}

// Return the AWS account id associated to the current set of credentials
func (ctx *resolveContext) getAWSAccountID() (string, error) {
	config, err := awshelper.CreateAwsConfig("", "")
	if err != nil {
		return "", err
	}

	identity, err := sts.NewFromConfig(*config).GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	return *identity.Account, nil
}

func (ctx *resolveContext) setGlobalVariable(key string, value interface{}) string {
	if key == "" {
		for key, value := range collections.AsDictionary(value).AsMap() {
			ctx.options.SetVariable(key, value, options.FunctionOverwrite)
		}
	} else {
		ctx.options.SetVariable(key, value, options.FunctionOverwrite)
	}
	return ""
}

// Convert the slice of cty values to a slice of strings. If any of the values in the given slice is not a string,
// return an error.
func ctySliceToStringSlice(args []cty.Value) ([]string, error) {
	var out []string
	for _, arg := range args {
		if arg.Type() != cty.String {
			return nil, tgerrors.WithStackTrace(invalidParameterType{Expected: "string", Actual: arg.Type().FriendlyName()})
		}
		out = append(out, arg.AsString())
	}
	return out, nil
}

type invalidParameterType struct {
	Expected string
	Actual   string
}

func (err invalidParameterType) Error() string {
	return fmt.Sprintf("Expected param of type %s but got %s", err.Expected, err.Actual)
}
