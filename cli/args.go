package cli

import (
	goErrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
)

// Parse command line options that are passed in for Terragrunt
func ParseTerragruntOptions(cliContext *cli.Context) (*options.TerragruntOptions, error) {
	terragruntOptions, err := parseTerragruntOptionsFromArgs(cliContext.Args())
	if err != nil {
		return nil, err
	}

	terragruntOptions.Writer = cliContext.App.Writer
	terragruntOptions.ErrWriter = cliContext.App.ErrWriter

	return terragruntOptions, nil
}

// TODO: replace the urfave CLI library with something else.
//
// EXPLANATION: The normal way to parse flags with the urfave CLI library would be to define the flags in the
// CreateTerragruntCLI method and to read the values of those flags using cliContext.String(...),
// cliContext.Bool(...), etc. Unfortunately, this does not work here due to a limitation in the urfave
// CLI library: if the user passes in any "command" whatsoever, (e.g. the "apply" in "terragrunt apply"), then
// any flags that come after it are not parsed (e.g. the "--foo" is not parsed in "terragrunt apply --foo").
// Therefore, we have to parse options ourselves, which is infuriating. For more details on this limitation,
// see: https://github.com/urfave/cli/issues/533. For now, our workaround is to dumbly loop over the arguments
// and look for the ones we need, but in the future, we should change to a different CLI library to avoid this
// limitation.
func parseTerragruntOptionsFromArgs(args []string) (*options.TerragruntOptions, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	workingDir, err := parseStringArg(args, OPT_WORKING_DIR, currentDir)
	if err != nil {
		return nil, err
	}

	terragruntConfigPath, err := parseStringArg(args, OPT_TERRAGRUNT_CONFIG, os.Getenv("TERRAGRUNT_CONFIG"))
	if err != nil {
		return nil, err
	}
	if terragruntConfigPath == "" {
		terragruntConfigPath = config.DefaultConfigPath(workingDir)
	}

	terraformPath, err := parseStringArg(args, OPT_TERRAGRUNT_TFPATH, os.Getenv("TERRAGRUNT_TFPATH"))
	if err != nil {
		return nil, err
	}
	if terraformPath == "" {
		terraformPath = "terraform"
	}

	terraformSource, err := parseStringArg(args, OPT_TERRAGRUNT_SOURCE, os.Getenv("TERRAGRUNT_SOURCE"))
	if err != nil {
		return nil, err
	}

	loggingLevel, err := parseStringArg(args, OPT_LOGGING_LEVEL, os.Getenv("TERRAGRUNT_LOGGING_LEVEL"))
	if err != nil {
		return nil, err
	}

	sourceUpdate := parseBooleanArg(args, OPT_TERRAGRUNT_SOURCE_UPDATE, false)

	ignoreDependencyErrors := parseBooleanArg(args, OPT_TERRAGRUNT_IGNORE_DEPENDENCY_ERRORS, false)

	opts := options.NewTerragruntOptions(filepath.ToSlash(terragruntConfigPath))
	opts.TerraformPath = filepath.ToSlash(terraformPath)
	opts.NonInteractive = parseBooleanArg(args, OPT_NON_INTERACTIVE, false)
	opts.TerraformCliArgs = filterTerragruntArgs(args)
	opts.WorkingDir = filepath.ToSlash(workingDir)
	opts.Logger = util.CreateLogger("")
	opts.RunTerragrunt = runTerragrunt
	opts.Source = terraformSource
	opts.SourceUpdate = sourceUpdate
	opts.IgnoreDependencyErrors = ignoreDependencyErrors
	opts.Env = map[string]string{}
	opts.Variables = options.VariableList{}

	parseEnvironmentVariables(opts, os.Environ())
	opts.TerraformCliArgs = filterVarsAndVarFiles(opts, opts.TerraformCliArgs)

	err = util.InitLogging(loggingLevel, logging.INFO, !util.ListContainsElement(opts.TerraformCliArgs, "-no-color"))
	return opts, err
}

func filterTerraformExtraArgs(terragruntOptions *options.TerragruntOptions, terragruntConfig *config.TerragruntConfig) []string {
	out := []string{}
	cmd := firstArg(terragruntOptions.TerraformCliArgs)

	for _, arg := range terragruntConfig.Terraform.ExtraArgs {
		currentCommandIncluded := util.ListContainsElement(arg.Commands, cmd)

		if currentCommandIncluded {
			out = append(out, arg.Arguments...)
		}

		// We first process all the -var because they have precedence over -var-file
		// If vars is specified, add -var <key=value> for each specified key
		keyFunc := func(key string) string { return strings.Split(key, "=")[0] }
		varList := util.RemoveDuplicatesFromList(arg.Vars, true, keyFunc)
		variablesExplicitlyProvided := terragruntOptions.VariablesExplicitlyProvided()
		for _, varDef := range varList {
			varDef = config.SubstituteVars(varDef, terragruntOptions)
			if key, value, err := splitVariable(varDef); err != nil {
				terragruntOptions.Logger.Warningf("-var ignored in %v: %v", arg.Name, err)
			} else {
				if util.ListContainsElement(variablesExplicitlyProvided, key) {
					continue
				}
				terragruntOptions.Variables.SetValue(key, value, options.VarParameter)
			}
			if currentCommandIncluded {
				out = append(out, "-var", varDef)
			}
		}

		// If RequiredVarFiles is specified, add -var-file=<file> for each specified files
		for _, file := range util.RemoveDuplicatesFromListKeepLast(arg.RequiredVarFiles) {
			file = config.SubstituteVars(file, terragruntOptions)
			importTfVarFile(terragruntOptions, file, options.VarFile)
			terragruntOptions.Logger.Infof("Importing %s", file)
			if currentCommandIncluded {
				out = append(out, fmt.Sprintf("-var-file=%s", file))
			}
		}

		// If OptionalVarFiles is specified, check for each file if it exists and if so, add -var-file=<file>
		// It is possible that many files resolve to the same path, so we remove duplicates.
		for _, file := range util.RemoveDuplicatesFromListKeepLast(arg.OptionalVarFiles) {
			file = config.SubstituteVars(file, terragruntOptions)
			if util.FileExists(file) {
				if currentCommandIncluded {
					out = append(out, fmt.Sprintf("-var-file=%s", file))
				}
				terragruntOptions.Logger.Infof("Importing %s", file)
				importTfVarFile(terragruntOptions, file, options.VarFile)
			} else if currentCommandIncluded {
				terragruntOptions.Logger.Infof("Skipping var-file %s as it does not exist", file)
			}
		}
	}

	return out
}

func parseEnvironmentVariables(terragruntOptions *options.TerragruntOptions, environment []string) {
	const tfPrefix = "TF_VAR_"
	for i := 0; i < len(environment); i++ {
		if key, value, err := splitVariable(environment[i]); err != nil {
			terragruntOptions.Logger.Warningf("Environment variable ignored: %v", err)
		} else {
			terragruntOptions.Env[key] = value
			// All environment variables starting with TF_ENV_ are considered as variables
			if strings.HasPrefix(key, tfPrefix) {
				terragruntOptions.Variables.SetValue(key[len(tfPrefix):], value, options.Environment)
			}
		}
	}
}

func splitVariable(str string) (key, value string, err error) {
	variableSplit := strings.SplitN(str, "=", 2)

	if len(variableSplit) == 2 {
		key, value, err = strings.TrimSpace(variableSplit[0]), variableSplit[1], nil
	} else {
		err = goErrors.New(fmt.Sprintf("Invalid variable format %v, should be name=value", str))
	}
	return
}

func importTfVarFile(terragruntOptions *options.TerragruntOptions, path string, source options.VariableSource) {
	vars, err := util.LoadTfVars(path)
	if err != nil {
		terragruntOptions.Logger.Errorf("Unable to read file %s, %v", path, err)
	}
	for key, value := range vars {
		terragruntOptions.Variables.SetValue(key, value, source)
	}
}

func filterVarsAndVarFiles(terragruntOptions *options.TerragruntOptions, args []string) []string {
	const varFile = "-var-file="
	const varArg = "-var"

	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], varFile) {
			path := args[i][len(varFile):]
			importTfVarFile(terragruntOptions, path, options.VarFileExplicit)
		}
	}

	for i := 0; i < len(args); i++ {
		if args[i] == varArg && i+1 < len(args) {
			if key, value, err := splitVariable(args[i+1]); err != nil {
				terragruntOptions.Logger.Warningf("-var ignored: %v", err)
			} else {
				terragruntOptions.Variables.SetValue(key, value, options.VarParameterExplicit)
			}
		}
	}

	if util.ListContainsElement(config.TERRAFORM_COMMANDS_NEED_VARS, firstArg(terragruntOptions.TerraformCliArgs)) {
		// The -var and -var-file are required by the terraform command, we return the args list unaltered
		return args
	}

	// We must remove the -var and -var-file arguments because they are not needed by the terraform command
	// but they may have been supplied by the user to help determine the current content
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], varFile) {
			continue
		}
		if args[i] == varArg && i+1 < len(args) {
			i++
			continue
		}
		filtered = append(filtered, args[i])
	}

	return filtered
}

// Return a copy of the given args with all Terragrunt-specific args removed
func filterTerragruntArgs(args []string) []string {
	out := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		argWithoutPrefix := strings.TrimPrefix(arg, "--")

		if util.ListContainsElement(MULTI_MODULE_COMMANDS, arg) {
			// Skip multi-module commands entirely
			continue
		}

		if util.ListContainsElement(ALL_TERRAGRUNT_STRING_OPTS, argWithoutPrefix) {
			// String flags have the argument and the value, so skip both
			i = i + 1
			continue
		}
		if util.ListContainsElement(ALL_TERRAGRUNT_BOOLEAN_OPTS, argWithoutPrefix) {
			// Just skip the boolean flag
			continue
		}

		out = append(out, arg)
	}
	return out
}

// Find a boolean argument (e.g. --foo) of the given name in the given list of arguments. If it's present, return true.
// If it isn't, return defaultValue.
func parseBooleanArg(args []string, argName string, defaultValue bool) bool {
	for _, arg := range args {
		if arg == fmt.Sprintf("--%s", argName) {
			return true
		}
	}
	return defaultValue
}

// Find a string argument (e.g. --foo "VALUE") of the given name in the given list of arguments. If it's present,
// return its value. If it is present, but has no value, return an error. If it isn't present, return defaultValue.
func parseStringArg(args []string, argName string, defaultValue string) (string, error) {
	for i, arg := range args {
		if arg == fmt.Sprintf("--%s", argName) {
			if (i + 1) < len(args) {
				return args[i+1], nil
			} else {
				return "", errors.WithStackTrace(ArgMissingValue(argName))
			}
		}
	}
	return defaultValue, nil
}

// A convenience method that returns the first item (0th index) in the given list or an empty string if this is an
// empty list
func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// A convenience method that returns the second item (1st index) in the given list or an empty string if this is a
// list that has less than 2 items in it
func secondArg(args []string) string {
	if len(args) > 1 {
		return args[1]
	}
	return ""
}

// Custom error types

type ArgMissingValue string

func (err ArgMissingValue) Error() string {
	return fmt.Sprintf("You must specify a value for the --%s option", string(err))
}
