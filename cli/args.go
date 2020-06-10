package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coveooss/multilogger/reutils"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// ParseTerragruntOptions parses command line options that are passed in for Terragrunt
func ParseTerragruntOptions(cliContext *cli.Context) (*options.TerragruntOptions, error) {
	terragruntOptions, err := parseTerragruntOptionsFromArgs(cliContext.Args())
	if err != nil {
		return nil, err
	}
	terragruntOptions.Writer = terragruntOptions.Logger.Copy().SetStdout(cliContext.App.Writer)
	terragruntOptions.ErrWriter = terragruntOptions.Logger.Copy().SetStdout(cliContext.App.ErrWriter)
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
		if err != nil {
			return
		}
		if result, err = parseStringArg(args, argName, ""); err == nil && result == "" {
			for _, def := range defaultValues {
				result = def
				if result != "" {
					break
				}
			}
		}
		return
	}

	parseList := func(argName string, defaultValues ...string) []string {
		result := parse(argName, defaultValues...)
		if err != nil {
			return nil
		}
		return util.RemoveElementFromList(strings.Split(result, string(",")), "")
	}

	workingDir := filepath.ToSlash(parse(optWorkingDir, currentDir))
	terragruntConfigPath := filepath.ToSlash(parse(optTerragruntConfig, os.Getenv(options.EnvConfig)))

	if !strings.Contains(terragruntConfigPath, "/") {
		terragruntConfigPath = filepath.ToSlash(util.JoinPath(workingDir, terragruntConfigPath))
	}
	terraformPath := parse(optTerragruntTFPath, os.Getenv(options.EnvTFPath), "terraform")

	opts := options.NewTerragruntOptions(terragruntConfigPath)
	opts.TerraformPath = filepath.ToSlash(terraformPath)
	opts.NonInteractive = parseBooleanArg(args, optNonInteractive, "", false)
	opts.TerraformCliArgs = filterTerragruntArgs(args)
	opts.WorkingDir = filepath.ToSlash(workingDir)
	opts.RunTerragrunt = runTerragrunt
	opts.Source = parse(optTerragruntSource, os.Getenv(options.EnvSource))
	opts.SourceUpdate = parseBooleanArg(args, optTerragruntSourceUpdate, options.EnvSourceUpdate, false)
	opts.IgnoreDependencyErrors = parseBooleanArg(args, optTerragruntIgnoreDependencyErrors, "", false)
	opts.AwsProfile = parse(optAWSProfile)
	opts.ApprovalHandler = parse(optApprovalHandler)
	opts.ApplyTemplate = parseBooleanArg(args, optApplyTemplate, options.EnvApplyTemplate, false)
	opts.TemplateAdditionalPatterns = parseList(optTemplatePatterns, os.Getenv(options.EnvTemplatePatterns))
	opts.BootConfigurationPaths = parseList(optBootConfigs, os.Getenv(options.EnvBootConfigs))
	opts.PreBootConfigurationPaths = parseList(optPreBootConfigs, os.Getenv(options.EnvPreBootConfigs))
	opts.CheckSourceFolders = !parseBooleanArg(args, optIncludeEmptyFolders, options.EnvIncludeEmptyFolders, false)

	flushDelay := parse(optFlushDelay, os.Getenv(options.EnvFlushDelay), "60s")
	nbWorkers := parse(optNbWorkers, os.Getenv(options.EnvWorkers), "10")
	loggingLevel := parse(optLoggingLevel, os.Getenv(options.EnvLoggingLevel), logrus.InfoLevel.String())
	fileLoggingDir := parse(optLoggingFileDir, os.Getenv(options.EnvLoggingFileDir))
	fileLoggingLevel := parse(optLoggingFileLevel, os.Getenv(options.EnvLoggingFileLevel), logrus.DebugLevel.String())

	if err != nil {
		return nil, err
	}

	if opts.RefreshOutputDelay, err = time.ParseDuration(flushDelay); err != nil {
		return nil, fmt.Errorf("refresh delay must be expressed with unit (i.e. 45s)")
	}

	if opts.NbWorkers, err = strconv.Atoi(nbWorkers); err != nil {
		return nil, fmt.Errorf("number of workers must be expressed as integer")
	}

	opts.Logger.SetDefaultConsoleHookLevel(loggingLevel)
	opts.Logger.SetColor(!util.ListContainsElement(opts.TerraformCliArgs, "-no-color"))
	if fileLoggingDir != "" {
		opts.Logger.AddFile(fileLoggingDir, true, fileLoggingLevel)
	}

	os.Setenv(options.EnvLoggingLevel, opts.Logger.GetHookLevel("").String())
	os.Setenv(options.EnvTFPath, terraformPath)

	parseEnvironmentVariables(opts, os.Environ())

	// We remove the -var and -var-file from the cli arguments if the target command does not require
	// those parameters. We have to get the cmd from the args since multi-module commands xxx-all are
	// stripped from the cli args.
	var cmd string
	if len(args) > 0 {
		cmd = strings.TrimSuffix(args[0], multiModuleSuffix)
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
	// https://regex101.com/r/9Gm4wt/2
	reVarFile := regexp.MustCompile(`^-{1,2}(?P<var>var(?P<file>-file)?)(?:=(?P<value>.*))?$`)
	var filteredArgs []string
	if !util.ListContainsElement(config.TerraformCommandWithVarFile, command) {
		filteredArgs = make([]string, 0, len(args))
	}

	for i := 0; i < len(args); i++ {
		if matches, match := reutils.MultiMatch(args[i], reVarFile); match >= 0 {
			if strings.HasPrefix(matches[""], "--") {
				// If the user specified is argument with --var or --var-file, we uniformize it to -var/-var-file
				args[i] = args[i][1:]
			}
			if matches["value"] == "" && i+1 < len(args) {
				// The value is specified in the next argument
				matches["value"] = args[i+1]
				i++
			}
			if matches["file"] == "" {
				// The value is a single variable to set
				key, value, err := util.SplitEnvVariable(matches["value"])
				if err != nil {
					return nil, err
				}
				terragruntOptions.SetVariable(key, convertToNativeType(value), options.VarParameterExplicit)
			} else {
				// The value represent a file to load
				vars, err := terragruntOptions.LoadVariablesFromFile(matches["value"])
				if err != nil {
					return nil, err
				}
				terragruntOptions.ImportVariablesMap(vars, options.VarFileExplicit)
			}
			if filteredArgs != nil {
				// We have to filter arguments, so we ignore the current var argument
				continue
			}
		}

		if filteredArgs != nil {
			// We have to filter arguments, so we append the non var argument to the list
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	if filteredArgs != nil {
		// We must remove the -var and -var-file arguments because they are not supported by the terraform command
		// but they may have been supplied by the user to help determine the current content
		return filteredArgs, nil
	}
	return args, nil
}

func convertToNativeType(s string) interface{} {
	if s := strings.TrimSpace(s); s != "" {
		if i, err := strconv.ParseInt(s, 10, 0); err == nil {
			return i
		} else if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		} else if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
	}
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) || strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`) {
		return s[1 : len(s)-1]
	}
	return s
}

// Return a copy of the given args with all Terragrunt-specific args removed
func filterTerragruntArgs(args []string) []string {
	out := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		argWithoutPrefix := strings.TrimPrefix(arg, "--")

		if strings.HasSuffix(arg, multiModuleSuffix) {
			continue
		}

		if util.ListContainsElement(allTerragruntStringOpts, argWithoutPrefix) {
			// String flags have the argument and the value, so skip both
			i = i + 1
			continue
		}
		if util.ListContainsElement(allTerragruntBooleanOpts, argWithoutPrefix) {
			// Just skip the boolean flag
			continue
		}

		out = append(out, arg)
	}
	return out
}

// Find a boolean argument (e.g. --foo) of the given name in the given list of arguments. If it's present, return true.
// If it isn't, return defaultValue.
func parseBooleanArg(args []string, argName string, envVar string, defaultValue bool) bool {
	if value, ok := os.LookupEnv(envVar); envVar != "" && ok {
		value = strings.ToLower(value)
		parsedValue, err := strconv.ParseBool(value)
		defaultValue = (parsedValue && err == nil) || value == "on" || value == "yes" || value == "y"
	}
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
	givenArg := fmt.Sprintf("--%s", argName)
	for i, arg := range args {
		if arg == givenArg {
			if (i + 1) < len(args) {
				return args[i+1], nil
			}
			return "", errors.WithStackTrace(ErrArgMissingValue(argName))
		}
	}
	return defaultValue, nil
}

// ErrArgMissingValue indicates that there is a missing argument value
type ErrArgMissingValue string

func (err ErrArgMissingValue) Error() string {
	return fmt.Sprintf("You must specify a value for the --%s option", string(err))
}
