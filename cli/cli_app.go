package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/coveo/gotemplate/utils"
	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/remote"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/urfave/cli"
)

const OPT_TERRAGRUNT_CONFIG = "terragrunt-config"
const OPT_TERRAGRUNT_TFPATH = "terragrunt-tfpath"
const OPT_APPROVAL_HANDLER = "terragrunt-approval"
const OPT_APPROVAL_CONFIG = "terragrunt-approval-config"
const OPT_NON_INTERACTIVE = "terragrunt-non-interactive"
const OPT_WORKING_DIR = "terragrunt-working-dir"
const OPT_TERRAGRUNT_SOURCE = "terragrunt-source"
const OPT_TERRAGRUNT_SOURCE_UPDATE = "terragrunt-source-update"
const OPT_TERRAGRUNT_IGNORE_DEPENDENCY_ERRORS = "terragrunt-ignore-dependency-errors"
const OPT_LOGGING_LEVEL = "terragrunt-logging-level"
const OPT_AWS_PROFILE = "profile"

var ALL_TERRAGRUNT_BOOLEAN_OPTS = []string{OPT_NON_INTERACTIVE, OPT_TERRAGRUNT_SOURCE_UPDATE, OPT_TERRAGRUNT_IGNORE_DEPENDENCY_ERRORS}
var ALL_TERRAGRUNT_STRING_OPTS = []string{OPT_TERRAGRUNT_CONFIG, OPT_TERRAGRUNT_TFPATH, OPT_WORKING_DIR, OPT_TERRAGRUNT_SOURCE, OPT_LOGGING_LEVEL, OPT_AWS_PROFILE, OPT_APPROVAL_HANDLER, OPT_APPROVAL_CONFIG}

const MULTI_MODULE_SUFFIX = "-all"
const CMD_INIT = "init"

// DEPRECATED_COMMANDS is a map of deprecated commands to the commands that replace them.
var DEPRECATED_COMMANDS = map[string]string{
	"spin-up":   "apply-all",
	"tear-down": "destroy-all",
}

var TERRAFORM_COMMANDS_THAT_USE_STATE = []string{
	"init",
	"apply",
	"destroy",
	"env",
	"import",
	"graph",
	"output",
	"plan",
	"push",
	"refresh",
	"show",
	"taint",
	"untaint",
	"validate",
	"force-unlock",
	"state",
}

var TERRAFORM_COMMANDS_WITH_SUBCOMMAND = []string{
	"debug",
	"state",
}

// Since Terragrunt is just a thin wrapper for Terraform, and we don't want to repeat every single Terraform command
// in its definition, we don't quite fit into the model of any Go CLI library. Fortunately, urfave/cli allows us to
// override the whole template used for the Usage Text.
//
// TODO: this description text has copy/pasted versions of many Terragrunt constants, such as command names and file
// names. It would be easy to make this code DRY using fmt.Sprintf(), but then it's hard to make the text align nicely.
// Write some code to take generate this help text automatically, possibly leveraging code that's part of urfave/cli.
const customUsageText = `DESCRIPTION:
   {{.Name}} - {{.UsageText}}

USAGE:
   {{.Usage}}

COMMANDS:
   get-doc [options...] [filters...] Print the documentation of all extra_arguments, import_files, pre_hooks, post_hooks and extra_command.
   get-versions                      Get all versions of underlying tools (including extra_command).
   get-stack [options]               Get the list of stack to execute sorted by dependency order.

   command --help | -h               Print the command detailed help 

   -all operations:
   plan-all                          Display the plans of a 'stack' by running 'terragrunt plan' in each subfolder (with a summary at the end).
   apply-all                         Apply a 'stack' by running 'terragrunt apply' in each subfolder.
   output-all                        Display the outputs of a 'stack' by running 'terragrunt output' in each subfolder (no error if a subfolder doesn't have outputs).
   destroy-all                       Destroy a 'stack' by running 'terragrunt destroy' in each subfolder in reverse dependency order.
   *-all                             In fact, the -all could be applied on any terraform or custom commands (that's cool).

   terraform commands:
   *             Terragrunt forwards all other commands directly to Terraform

GLOBAL OPTIONS:
   terragrunt-config                    Path to the Terragrunt config file. Default is terraform.tfvars.
   terragrunt-tfpath                    Path to the Terraform binary. Default is terraform (on PATH).
   terragrunt-non-interactive           Assume "yes" for all prompts.
   terragrunt-working-dir               The path to the Terraform templates. Default is current directory.
   terragrunt-source                    Download Terraform configurations from the specified source into a temporary folder, and run Terraform in that temporary folder.
   terragrunt-source-update             Delete the contents of the temporary folder to clear out any old, cached source code before downloading new source code into it.
   terragrunt-ignore-dependency-errors  *-all commands continue processing components even if a dependency fails.
   terragrunt-logging-level             CRITICAL (0), ERROR (1), WARNING (2), NOTICE (3), INFO (4), DEBUG (5).
   profile                              Specify an AWS profile to use.

VERSION:
   {{.Version}}{{if len .Authors}}

AUTHOR(S):
   {{range .Authors}}{{.}}{{end}}
   {{end}}
`

var MODULE_REGEX = regexp.MustCompile(`module[[:blank:]]+".+"`)

// This uses the constraint syntax from https://github.com/hashicorp/go-version
const DEFAULT_TERRAFORM_VERSION_CONSTRAINT = ">= v0.9.3"

const TERRAFORM_EXTENSION_GLOB = "*.tf"

var terragruntVersion string
var terraformVersion string

// Create the Terragrunt CLI App
func CreateTerragruntCli(version string, writer io.Writer, errwriter io.Writer) *cli.App {
	cli.OsExiter = func(exitCode int) {
		// Do nothing. We just need to override this function, as the default value calls os.Exit, which
		// kills the app (or any automated test) dead in its tracks.
	}

	cli.AppHelpTemplate = customUsageText

	app := cli.NewApp()

	app.Name = "terragrunt"
	app.Author = "Gruntwork <www.gruntwork.io>"
	app.Version = version
	app.Action = runApp
	app.Usage = "terragrunt <COMMAND>"
	app.Writer = writer
	app.ErrWriter = errwriter
	app.UsageText = `Terragrunt is a thin wrapper for Terraform that provides extra tools for working with multiple
   Terraform modules, remote state, and locking. For documentation, see https://github.com/gruntwork-io/terragrunt/.`

	terragruntVersion = version
	return app
}

// The sole action for the app
func runApp(cliContext *cli.Context) (finalErr error) {
	defer errors.Recover(func(cause error) { finalErr = cause })

	os.Setenv("TERRAGRUNT_CACHE_FOLDER", util.GetTempDownloadFolder("terragrunt-cache"))
	os.Setenv("TERRAGRUNT_ARGS", strings.Join(os.Args, " "))

	terragruntOptions, err := ParseTerragruntOptions(cliContext)
	if err != nil {
		return err
	}

	// If someone calls us with no args at all, show the help text and exit
	if !cliContext.Args().Present() {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		terragruntOptions.Println(title("\nTERRAGRUNT\n"))
		cli.ShowAppHelp(cliContext)

		fmt.Fprintln(cliContext.App.Writer)
		util.SetWarningLoggingLevel()
		terragruntOptions.Println(title("TERRAFORM\n"))
		shell.RunTerraformCommand(terragruntOptions, "--help")
		return nil
	}

	// If AWS is configured, we init the session to ensure that proper environment variables are set
	if terragruntOptions.AwsProfile != "" || os.Getenv("AWS_PROFILE") != "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		_, err := aws_helper.InitAwsSession(terragruntOptions.AwsProfile)
		if err != nil {
			return err
		}
	}

	if err := CheckTerraformVersion(DEFAULT_TERRAFORM_VERSION_CONSTRAINT, terragruntOptions); err != nil {
		return err
	}

	givenCommand := cliContext.Args().First()
	command := checkDeprecated(givenCommand, terragruntOptions)
	return runCommand(command, terragruntOptions)
}

// checkDeprecated checks if the given command is deprecated.  If so: prints a message and returns the new command.
func checkDeprecated(command string, terragruntOptions *options.TerragruntOptions) string {
	newCommand, deprecated := DEPRECATED_COMMANDS[command]
	if deprecated {
		terragruntOptions.Logger.Warningf("%v is deprecated; running %v instead.\n", command, newCommand)
		return newCommand
	}
	return command
}

// runCommand runs one or many terraform commands based on the type of
// terragrunt command
func runCommand(command string, terragruntOptions *options.TerragruntOptions) (finalEff error) {
	terragruntOptions.IgnoreRemainingInterpolation = true
	if err := setRoleEnvironmentVariables(terragruntOptions, ""); err != nil {
		return err
	}
	if command == getStackCommand || strings.HasSuffix(command, MULTI_MODULE_SUFFIX) {
		return runMultiModuleCommand(command, terragruntOptions)
	}
	return runTerragrunt(terragruntOptions)
}

var runHandler func(*options.TerragruntOptions, *config.TerragruntConfig) error

// Run Terragrunt with the given options and CLI args. This will forward all the args directly to Terraform, enforcing
// best practices along the way.
func runTerragrunt(terragruntOptions *options.TerragruntOptions) (result error) {
	terragruntOptions.IgnoreRemainingInterpolation = true
	conf, err := config.ReadTerragruntConfig(terragruntOptions)
	if err != nil {
		return err
	}

	// If runHandler has been specified, we bypass the planned execution and defer the control // to the handler
	if runHandler != nil {
		return runHandler(terragruntOptions, conf)
	}

	// Check if the current command is an extra command
	actualCommand := conf.ExtraCommands.ActualCommand(terragruntOptions.TerraformCliArgs[0])

	if conf.Terraform != nil && len(conf.Terraform.ExtraArgs) > 0 {
		commandLength := 1
		if util.ListContainsElement(TERRAFORM_COMMANDS_WITH_SUBCOMMAND, terragruntOptions.TerraformCliArgs[0]) {
			commandLength = 2
		}

		// Options must be inserted after command but before the other args command is either 1 word or 2 words
		var args []string
		args = append(args, terragruntOptions.TerraformCliArgs[:commandLength]...)
		args = append(args, filterTerraformExtraArgs(terragruntOptions, conf)...)
		if commandLength <= len(terragruntOptions.TerraformCliArgs) {
			args = append(args, terragruntOptions.TerraformCliArgs[commandLength:]...)
		}
		terragruntOptions.TerraformCliArgs = args
	}

	conf.SubstituteAllVariables(terragruntOptions, false)

	// Copy the deployment files to the working directory
	sourceURL, hasSourceURL := getTerraformSourceURL(terragruntOptions, conf)
	if sourceURL == "" {
		sourceURL = terragruntOptions.WorkingDir
	}

	if conf.Uniqueness != nil {
		// If uniqueness_criteria has been defined, we set it in the options to ensure that
		// we use distinct folder based on this criteria
		terragruntOptions.Uniqueness = *conf.Uniqueness
	}
	terraformSource, err := processTerraformSource(sourceURL, terragruntOptions)
	if err != nil {
		return err
	}
	if hasSourceURL || len(conf.ImportFiles) > 0 {
		// If there are import files, we force the usage of a temp directory.
		if err = downloadTerraformSource(terraformSource, terragruntOptions); err != nil {
			return err
		}
	}
	conf.SubstituteAllVariables(terragruntOptions, true)

	// Import the required files in the temporary folder and copy the temporary imported file in the
	// working folder. We did not put them directly into the folder because terraform init would complain
	// if there are already terraform files in the target folder
	if _, err := conf.ImportFiles.Run(); err != nil {
		return err
	}

	// Retrieve the default variables from the terraform files
	err = importDefaultVariables(terragruntOptions, terragruntOptions.WorkingDir)
	if err != nil {
		return err
	}

	terragruntOptions.IgnoreRemainingInterpolation = false
	conf.SubstituteAllVariables(terragruntOptions, true)

	if actualCommand.Command == "get-versions" {
		PrintVersions(terragruntOptions, conf)
		return
	}

	if actualCommand.Command == "get-doc" {
		PrintDoc(terragruntOptions, conf)
		return
	}

	// Check if we must configure environment variables to assume a distinct role when applying external commands.
	if conf.AssumeRole != nil && *conf.AssumeRole != "" {
		terragruntOptions.Logger.Notice("Assuming role", *conf.AssumeRole)
		if err := setRoleEnvironmentVariables(terragruntOptions, *conf.AssumeRole); err != nil {
			return err
		}
	}

	terragruntOptions.Env["TERRAGRUNT_COMMAND"] = terragruntOptions.TerraformCliArgs[0]
	if actualCommand.Extra != nil {
		terragruntOptions.Env["TERRAGRUNT_EXTRA_COMMAND"] = actualCommand.Command
	}
	terragruntOptions.Env["TERRAGRUNT_VERSION"] = terragruntVersion
	terragruntOptions.Env["TERRAFORM_VERSION"] = terraformVersion

	// Temporary make the command behave as another command to initialize the folder properly
	// (to be sure that the remote state file get initialized)
	if actualCommand.BehaveAs != "" {
		terragruntOptions.TerraformCliArgs[0] = actualCommand.BehaveAs
	}

	if err := downloadModules(terragruntOptions); err != nil {
		return err
	}

	if _, err := conf.ImportFiles.RunOnModules(); err != nil {
		return err
	}

	// If there is no terraform file in the folder, we skip the command
	tfFiles, err := utils.FindFiles(terragruntOptions.WorkingDir, false, false, "*.tf", "*.tf.json")
	if err != nil {
		return err
	}
	if len(tfFiles) == 0 {
		terragruntOptions.Logger.Warning("No terraform file found, skipping folder")
		return nil
	}

	// Save all variable files requested in the terragrunt config
	terragruntOptions.SaveVariables()

	// Set the temporary script folder as the first item of the PATH
	terragruntOptions.Env["PATH"] = fmt.Sprintf("%s%c%s", filepath.Join(terraformSource.WorkingDir, config.TerragruntScriptFolder), filepath.ListSeparator, terragruntOptions.Env["PATH"])

	// Executing the pre-hooks commands that should be ran before init state if there are
	if _, err = conf.PreHooks.Run(config.BeforeInitState); err != nil {
		return err
	}

	// Configure remote state if required
	if conf.RemoteState != nil {
		if err := configureRemoteState(conf.RemoteState, terragruntOptions); err != nil {
			return err
		}
	}

	// Executing the pre-hooks that should be ran after init state if there are
	if _, err = conf.PreHooks.Run(config.AfterInitState); err != nil {
		return err
	}

	defer func() {
		// If there is an error but it is in fact a plan status, we run the post hooks normally
		_, planStatusError := result.(errors.PlanWithChanges)

		// Executing the post-hooks commands if there are and there is no error
		if result == nil || planStatusError {
			if _, err := conf.PostHooks.Run(); err != nil {
				result = err
			}
		}
	}()

	// We define a filter to trap plan exit code that are not real error
	filterPlanError := func(err error, command string) error {
		if err == nil || command != "plan" {
			return err
		}
		if exiterr, ok := errors.Unwrap(err).(*exec.ExitError); ok {
			// For plan, an error with exit code 2 should not be considered as a real error
			if exiterr.Sys().(syscall.WaitStatus).ExitStatus() == errors.CHANGE_EXIT_CODE {
				return errors.PlanWithChanges{}
			}
		}
		return err
	}

	if actualCommand.Extra != nil {
		// The command is not a native terraform command
		runner := shell.RunShellCommand
		if *actualCommand.Extra.ExpandArgs {
			runner = shell.RunShellCommandExpandArgs
		}

		err = runner(terragruntOptions, actualCommand.Command, append(actualCommand.Extra.Arguments, terragruntOptions.TerraformCliArgs[1:]...)...)
		return filterPlanError(err, actualCommand.Extra.ActAs)
	}

	// If the command is 'init', stop here. That's because ConfigureRemoteState above will have already called
	// terraform init if it was necessary, and the below RunTerraformCommand would end up calling init without
	// the correct remote state arguments, which is confusing.
	if terragruntOptions.TerraformCliArgs[0] == CMD_INIT {
		terragruntOptions.Logger.Warning("Running 'init' manually is not necessary: Terragrunt will call it automatically when needed before running other Terraform commands")
		return nil
	}

	// We restore back the name of the command since it may have been temporary changed to support state file initialization and get modules
	terragruntOptions.TerraformCliArgs[0] = actualCommand.Command
	err = shell.RunTerraformCommand(terragruntOptions, terragruntOptions.TerraformCliArgs...)
	return filterPlanError(err, actualCommand.Command)
}

// Returns true if the command the user wants to execute is supposed to affect multiple Terraform modules, such as the
// apply-all or destroy-all command.
func isMultiModuleCommand(command string) bool {
	return strings.HasSuffix(command, MULTI_MODULE_SUFFIX)
}

// Execute a command that affects multiple Terraform modules, such as the apply-all or destroy-all command.
func runMultiModuleCommand(command string, terragruntOptions *options.TerragruntOptions) error {
	realCommand := strings.TrimSuffix(command, MULTI_MODULE_SUFFIX)
	if command == getStackCommand {
		return getStack(terragruntOptions)
	} else if strings.HasPrefix(command, "plan-") {
		return planAll(realCommand, terragruntOptions)
	} else if strings.HasPrefix(command, "apply-") {
		return applyAll(realCommand, terragruntOptions)
	} else if strings.HasPrefix(command, "destroy-") {
		return destroyAll(realCommand, terragruntOptions)
	} else if strings.HasPrefix(command, "output-") {
		return outputAll(realCommand, terragruntOptions)
	} else if strings.HasSuffix(command, MULTI_MODULE_SUFFIX) {
		return runAll(realCommand, terragruntOptions)
	}
	return errors.WithStackTrace(UnrecognizedCommand(command))
}

// A quick sanity check that calls `terraform get` to download modules, if they aren't already downloaded.
func downloadModules(terragruntOptions *options.TerragruntOptions) error {
	switch firstArg(terragruntOptions.TerraformCliArgs) {
	case "apply", "destroy", "graph", "output", "plan", "show", "taint", "untaint", "validate":
		shouldDownload, err := shouldDownloadModules(terragruntOptions)
		if err != nil {
			return err
		}
		if shouldDownload {
			return shell.RunTerraformCommandAndRedirectOutputToLogger(terragruntOptions, "get", "-update")
		}
	}

	return nil
}

// Return true if modules aren't already downloaded and the Terraform templates in this project reference modules.
// Note that to keep the logic in this code very simple, this code ONLY detects the case where you haven't downloaded
// modules at all. Detecting if your downloaded modules are out of date (as opposed to missing entirely) is more
// complicated and not something we handle at the moment.
func shouldDownloadModules(terragruntOptions *options.TerragruntOptions) (bool, error) {
	modulesPath := util.JoinPath(terragruntOptions.WorkingDir, ".terraform/modules")
	if util.FileExists(modulesPath) {
		return false, nil
	}

	return util.Grep(MODULE_REGEX, fmt.Sprintf("%s/%s", terragruntOptions.WorkingDir, TERRAFORM_EXTENSION_GLOB))
}

// If the user entered a Terraform command that uses state (e.g. plan, apply), make sure remote state is configured
// before running the command.
func configureRemoteState(remoteState *remote.RemoteState, terragruntOptions *options.TerragruntOptions) error {
	// We only configure remote state for the commands that use the tfstate files. We do not configure it for
	// commands such as "get" or "version".
	if util.ListContainsElement(TERRAFORM_COMMANDS_THAT_USE_STATE, firstArg(terragruntOptions.TerraformCliArgs)) {
		return remoteState.ConfigureRemoteState(terragruntOptions)
	}

	return nil
}

// runAll run the specified command on all configuration in a stack, in the order
// specified in the terraform_remote_state dependencies
func runAll(command string, terragruntOptions *options.TerragruntOptions) error {
	stack, err := configstack.FindStackInSubfolders(terragruntOptions)
	if err != nil {
		return err
	}

	terragruntOptions.Logger.Notice(stack)
	return stack.RunAll([]string{command}, terragruntOptions, configstack.NormalOrder)
}

// planAll prints the plans from all configuration in a stack, in the order
// specified in the terraform_remote_state dependencies
func planAll(command string, terragruntOptions *options.TerragruntOptions) error {
	stack, err := configstack.FindStackInSubfolders(terragruntOptions)
	if err != nil {
		return err
	}

	terragruntOptions.Logger.Notice(stack.String())
	return stack.Plan(command, terragruntOptions)
}

// Spin up an entire "stack" by running 'terragrunt apply' in each subfolder, processing them in the right order based
// on terraform_remote_state dependencies.
func applyAll(command string, terragruntOptions *options.TerragruntOptions) error {
	stack, err := configstack.FindStackInSubfolders(terragruntOptions)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf("%s\nAre you sure you want to run 'terragrunt apply' in each folder of the stack described above?", stack)
	shouldApplyAll, err := shell.PromptUserForYesNo(prompt, terragruntOptions)
	if err != nil {
		return err
	}

	if shouldApplyAll {
		return stack.RunAll([]string{command, "-input=false"}, terragruntOptions, configstack.NormalOrder)
	}

	return nil
}

// Tear down an entire "stack" by running 'terragrunt destroy' in each subfolder, processing them in the right order
// based on terraform_remote_state dependencies.
func destroyAll(command string, terragruntOptions *options.TerragruntOptions) error {
	stack, err := configstack.FindStackInSubfolders(terragruntOptions)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf("%s\nWARNING: Are you sure you want to run `terragrunt destroy` in each folder of the stack described above? There is no undo!", stack)
	shouldDestroyAll, err := shell.PromptUserForYesNo(prompt, terragruntOptions)
	if err != nil {
		return err
	}

	if shouldDestroyAll {
		return stack.RunAll([]string{command, "-force", "-input=false"}, terragruntOptions, configstack.ReverseOrder)
	}

	return nil
}

// outputAll prints the outputs from all configuration in a stack, in the order
// specified in the terraform_remote_state dependencies
func outputAll(command string, terragruntOptions *options.TerragruntOptions) error {
	stack, err := configstack.FindStackInSubfolders(terragruntOptions)
	if err != nil {
		return err
	}

	terragruntOptions.Logger.Notice(stack)
	return stack.Output(command, terragruntOptions)
}

// Custom error types

var DontManuallyConfigureRemoteState = fmt.Errorf("Instead of manually using the 'remote config' command, define your remote state settings in %s and Terragrunt will automatically configure it for you (and all your team members) next time you run it.", config.DefaultTerragruntConfigPath)

type UnrecognizedCommand string

func (commandName UnrecognizedCommand) Error() string {
	return fmt.Sprintf("Unrecognized command: %s", string(commandName))
}
