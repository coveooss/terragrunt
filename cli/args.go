package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
)

// ParseTerragruntOptions parses command line options that are passed in for Terragrunt
func ParseTerragruntOptions(cliContext *cli.Context) (*options.TerragruntOptions, error) {
	terragruntOptions, err := parseTerragruntOptionsFromArgs(cliContext.Args())
	if err != nil {
		return nil, err
	}

	terragruntOptions.Writer = util.LogCatcher{
		Writer: cliContext.App.Writer,
		Logger: terragruntOptions.Logger,
	}
	terragruntOptions.ErrWriter = util.LogCatcher{
		Writer: cliContext.App.ErrWriter,
		Logger: terragruntOptions.Logger,
	}

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

	parse := func(argName string, defaultValues ...string) (result string) {
		if err == nil {
			if result, err = parseStringArg(args, argName, ""); err == nil && result == "" {
				for _, def := range defaultValues {
					result = def
					if result != "" {
						break
					}
				}
			}

		}
		return
	}

	workingDir := parse(OPT_WORKING_DIR, currentDir)
	terragruntConfigPath := parse(OPT_TERRAGRUNT_CONFIG, os.Getenv(options.EnvConfig), config.DefaultConfigPath(workingDir))
	terraformPath := parse(OPT_TERRAGRUNT_TFPATH, os.Getenv(options.EnvTFPath), "terraform")
	terraformSource := parse(OPT_TERRAGRUNT_SOURCE, os.Getenv(options.EnvSource))
	loggingLevel := parse(OPT_LOGGING_LEVEL, os.Getenv(options.EnvLoggingLevel))
	awsProfile := parse(OPT_AWS_PROFILE)
	approvalHandler := parse(OPT_APPROVAL_HANDLER)
	sourceUpdate := parseBooleanArg(args, OPT_TERRAGRUNT_SOURCE_UPDATE, false)
	ignoreDependencyErrors := parseBooleanArg(args, OPT_TERRAGRUNT_IGNORE_DEPENDENCY_ERRORS, false)
	flushDelay := parse(OPT_FLUSH_DELAY, os.Getenv(options.EnvFlushDelay), "60s")
	nbWorkers := parse(OPT_NB_WORKERS, os.Getenv(options.EnvWorkers), "10")

	if err != nil {
		return nil, err
	}

	opts := options.NewTerragruntOptions(filepath.ToSlash(terragruntConfigPath))
	opts.TerraformPath = filepath.ToSlash(terraformPath)
	opts.NonInteractive = parseBooleanArg(args, OPT_NON_INTERACTIVE, false)
	opts.TerraformCliArgs = filterTerragruntArgs(args)
	opts.WorkingDir = filepath.ToSlash(workingDir)
	opts.RunTerragrunt = runTerragrunt
	opts.Source = terraformSource
	opts.SourceUpdate = sourceUpdate
	opts.IgnoreDependencyErrors = ignoreDependencyErrors
	opts.AwsProfile = awsProfile
	opts.ApprovalHandler = approvalHandler

	if opts.RefreshOutputDelay, err = time.ParseDuration(flushDelay); err != nil {
		return nil, fmt.Errorf("Refresh delay must be expressed with unit (i.e. 45s)")
	}

	if opts.NbWorkers, err = strconv.Atoi(nbWorkers); err != nil {
		return nil, fmt.Errorf("Number of workers must be expressed as integer")
	}

	level, err := util.InitLogging(loggingLevel, logging.NOTICE, !util.ListContainsElement(opts.TerraformCliArgs, "-no-color"))
	os.Setenv(options.EnvLoggingLevel, fmt.Sprintf("%d", level))
	os.Setenv(options.EnvTFPath, terraformPath)

	parseEnvironmentVariables(opts, os.Environ())

	// We remove the -var and -var-file from the cli arguments if the target command does not require
	// those parameters. We have to get the cmd from the args since multi-module commands xxx-all are
	// stripped from the cli args.
	var cmd string
	if len(args) > 0 {
		cmd = args[0]
		if strings.HasSuffix(cmd, MULTI_MODULE_SUFFIX) {
			cmd = strings.TrimSuffix(cmd, MULTI_MODULE_SUFFIX)
		}
	}
	opts.TerraformCliArgs, err = filterVarsAndVarFiles(cmd, opts, opts.TerraformCliArgs)

	return opts, err
}

func parseEnvironmentVariables(terragruntOptions *options.TerragruntOptions, environment []string) {
	const tfPrefix = "TF_VAR_"
	for i := 0; i < len(environment); i++ {
		if key, value, err := util.SplitEnvVariable(environment[i]); err != nil {
			terragruntOptions.Logger.Warning("Environment variable ignored:", environment[i], err)
		} else {
			terragruntOptions.Env[key] = value
			// All environment variables starting with TF_ENV_ are considered as variables
			if strings.HasPrefix(key, tfPrefix) {
				terragruntOptions.SetVariable(key[len(tfPrefix):], value, options.Environment)
			}
		}
	}
}

func filterVarsAndVarFiles(command string, terragruntOptions *options.TerragruntOptions, args []string) ([]string, error) {
	const varFile = "-var-file="
	const varArg = "-var"

	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], varFile) {
			path := args[i][len(varFile):]
			if err := terragruntOptions.ImportVariablesFromFile(path, options.VarFileExplicit); err != nil {
				return nil, err
			}
		}
	}

	for i := 0; i < len(args); i++ {
		if args[i] == varArg && i+1 < len(args) {
			if key, value, err := util.SplitEnvVariable(args[i+1]); err != nil {
				terragruntOptions.Logger.Warning("-var ignored:", args[i+1], err)
			} else {
				terragruntOptions.SetVariable(key, value, options.VarParameterExplicit)
			}
		}
	}

	if util.ListContainsElement(config.TerraformCommandWithVarFile, command) {
		// The -var and -var-file are required by the terraform command, we return the args list unaltered
		return args, nil
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

	return filtered, nil
}

func extractVarArgs() []string {
	var commandLineArgs []string
	for i := range os.Args {
		if os.Args[i] == "-var" {
			commandLineArgs = append(commandLineArgs, os.Args[i:i+2]...)
		}
	}
	return commandLineArgs
}

// Return a copy of the given args with all Terragrunt-specific args removed
func filterTerragruntArgs(args []string) []string {
	out := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		argWithoutPrefix := strings.TrimPrefix(arg, "--")

		if strings.HasSuffix(arg, MULTI_MODULE_SUFFIX) {
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
	argName = fmt.Sprintf("--%s", argName)
	for _, arg := range args {
		if arg == argName {
			return true
		}
	}
	return defaultValue
}

// Find a string argument (e.g. --foo "VALUE") of the given name in the given list of arguments. If it's present,
// return its value. If it is present, but has no value, return an error. If it isn't present, return defaultValue.
func parseStringArg(args []string, argName string, defaultValue string) (string, error) {
	argName = fmt.Sprintf("--%s", argName)
	for i, arg := range args {
		if arg == argName {
			if (i + 1) < len(args) {
				return args[i+1], nil
			}
			return "", errors.WithStackTrace(ArgMissingValue(argName))
		}
	}
	return defaultValue, nil
}

// Custom error types

type ArgMissingValue string

func (err ArgMissingValue) Error() string {
	return fmt.Sprintf("You must specify a value for the --%s option", string(err))
}
