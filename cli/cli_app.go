package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

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
const OPT_NON_INTERACTIVE = "terragrunt-non-interactive"
const OPT_WORKING_DIR = "terragrunt-working-dir"
const OPT_TERRAGRUNT_SOURCE = "terragrunt-source"
const OPT_TERRAGRUNT_SOURCE_UPDATE = "terragrunt-source-update"
const OPT_TERRAGRUNT_IGNORE_DEPENDENCY_ERRORS = "terragrunt-ignore-dependency-errors"
const OPT_LOGGING_LEVEL = "terragrunt-logging-level"
const OPT_AWS_PROFILE = "profile"

var ALL_TERRAGRUNT_BOOLEAN_OPTS = []string{OPT_NON_INTERACTIVE, OPT_TERRAGRUNT_SOURCE_UPDATE, OPT_TERRAGRUNT_IGNORE_DEPENDENCY_ERRORS}
var ALL_TERRAGRUNT_STRING_OPTS = []string{OPT_TERRAGRUNT_CONFIG, OPT_TERRAGRUNT_TFPATH, OPT_WORKING_DIR, OPT_TERRAGRUNT_SOURCE, OPT_LOGGING_LEVEL, OPT_AWS_PROFILE}

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
var CUSTOM_USAGE_TEXT = `DESCRIPTION:
   {{.Name}} - {{.UsageText}}

USAGE:
   {{.Usage}}

COMMANDS:
   plan-all             Display the plans of a 'stack' by running 'terragrunt plan' in each subfolder
   apply-all            Apply a 'stack' by running 'terragrunt apply' in each subfolder
   output-all           Display the outputs of a 'stack' by running 'terragrunt output' in each subfolder
   destroy-all          Destroy a 'stack' by running 'terragrunt destroy' in each subfolder
   get-all              Get all modules of a 'stack' by running 'terragrunt get' in each subfolder
   *                    Terragrunt forwards all other commands directly to Terraform

GLOBAL OPTIONS:
   terragrunt-config                    Path to the Terragrunt config file. Default is terraform.tfvars.
   terragrunt-tfpath                    Path to the Terraform binary. Default is terraform (on PATH).
   terragrunt-non-interactive           Assume "yes" for all prompts.
   terragrunt-working-dir               The path to the Terraform templates. Default is current directory.
   terragrunt-source                    Download Terraform configurations from the specified source into a temporary folder, and run Terraform in that temporary folder.
   terragrunt-source-update             Delete the contents of the temporary folder to clear out any old, cached source code before downloading new source code into it.
   terragrunt-ignore-dependency-errors  *-all commands continue processing components even if a dependency fails.
   terragrunt-logging-level             CRITICAL (0), ERROR (1), WARNING (2), NOTICE (3), INFO (4), DEBUG (5)
   profile                              Specify an AWS profile to use

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

// Create the Terragrunt CLI App
func CreateTerragruntCli(version string, writer io.Writer, errwriter io.Writer) *cli.App {
	cli.OsExiter = func(exitCode int) {
		// Do nothing. We just need to override this function, as the default value calls os.Exit, which
		// kills the app (or any automated test) dead in its tracks.
	}

	cli.AppHelpTemplate = CUSTOM_USAGE_TEXT

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

	return app
}

// The sole action for the app
func runApp(cliContext *cli.Context) (finalErr error) {
	defer errors.Recover(func(cause error) { finalErr = cause })

	terragruntOptions, err := ParseTerragruntOptions(cliContext)
	if err != nil {
		return err
	}

	// If someone calls us with no args at all, show the help text and exit
	if !cliContext.Args().Present() {
		cli.ShowAppHelp(cliContext)

		fmt.Fprintln(cliContext.App.Writer)
		util.SetWarningLoggingLevel()
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
	if strings.HasSuffix(command, MULTI_MODULE_SUFFIX) {
		return runMultiModuleCommand(command, terragruntOptions)
	}
	return runTerragrunt(terragruntOptions)
}

// Run Terragrunt with the given options and CLI args. This will forward all the args directly to Terraform, enforcing
// best practices along the way.
func runTerragrunt(terragruntOptions *options.TerragruntOptions) (result error) {
	terragruntOptions.IgnoreRemainingInterpolation = true
	conf, err := config.ReadTerragruntConfig(terragruntOptions)
	if err != nil {
		return err
	}

	// Check if the current command is an extra command
	extraCommand, behaveAs, extraArgs := getExtraCommand(terragruntOptions, conf)

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
	if err := importFiles(terragruntOptions, conf.ImportFiles, terragruntOptions.WorkingDir, false); err != nil {
		return err
	}

	// Retrieve the default variables from the terraform files
	err = importDefaultVariables(terragruntOptions, terragruntOptions.WorkingDir)
	if err != nil {
		return err
	}

	terragruntOptions.IgnoreRemainingInterpolation = false
	conf.SubstituteAllVariables(terragruntOptions, true)

	// Check if we must configure environment variables to assume a distinct role when applying external commands.
	if conf.AssumeRole != nil && *conf.AssumeRole != "" {
		terragruntOptions.Logger.Notice("Assuming role", *conf.AssumeRole)
		if err := setRoleEnvironmentVariables(terragruntOptions, *conf.AssumeRole); err != nil {
			return err
		}
	}

	terragruntOptions.Env["TERRAGRUNT_COMMAND"] = terragruntOptions.TerraformCliArgs[0]
	terragruntOptions.Env["TERRAGRUNT_EXTRA_COMMAND"] = extraCommand
	terragruntOptions.Env["TERRAGRUNT_TFPATH"] = terragruntOptions.TerraformPath

	// Temporary make the command behave as another command to initialize the folder properly
	// (to be sure that the remote state file get initialized)
	if behaveAs != "" {
		terragruntOptions.TerraformCliArgs[0] = behaveAs
	}

	if err := downloadModules(terragruntOptions); err != nil {
		return err
	}

	// Resolve the links to check if we must copy files in them
	modules, err := getModulesFolders(terragruntOptions)
	if err != nil {
		return err
	}
	for _, moduleFolder := range modules {
		if err := importFiles(terragruntOptions, conf.ImportFiles, moduleFolder, true); err != nil {
			return err
		}
	}

	// If there is no terraform file in the folder, we skip the command
	tfFiles, err := util.FindFiles(terragruntOptions.WorkingDir, "*.tf", "*.tf.json")
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
	if err = runHooks(terragruntOptions, conf.PreHooks, func(hook config.Hook) bool { return !hook.AfterInitState }); err != nil {
		return err
	}

	// Configure remote state if required
	if conf.RemoteState != nil {
		if err := configureRemoteState(conf.RemoteState, terragruntOptions); err != nil {
			return err
		}
	}

	// Executing the pre-hooks that should be ran after init state if there are
	if err = runHooks(terragruntOptions, conf.PreHooks, func(hook config.Hook) bool { return hook.AfterInitState }); err != nil {
		return err
	}

	defer func() {
		// Executing the post-hooks commands if there are and there is no error
		if result == nil {
			if err := runHooks(terragruntOptions, conf.PostHooks, nil); err != nil {
				result = err
			}
		}
	}()

	// If the command is 'init', stop here. That's because ConfigureRemoteState above will have already called
	// terraform init if it was necessary, and the below RunTerraformCommand would end up calling init without
	// the correct remote state arguments, which is confusing.
	if terragruntOptions.TerraformCliArgs[0] == CMD_INIT {
		terragruntOptions.Logger.Warning("Running 'init' manually is not necessary: Terragrunt will call it automatically when needed before running other Terraform commands")
		return nil
	}

	// Check if the supplied command is an extra command. In that case, we execute that command instead of calling terraform.
	if extraCommand != "" {
		// We restore back the name of the command since it may have been temporary changed to support
		// state file initialization and get modules
		terragruntOptions.TerraformCliArgs[0] = extraCommand
		return shell.RunShellCommand(terragruntOptions, extraCommand, append(extraArgs, terragruntOptions.TerraformCliArgs[1:]...)...)
	}

	return shell.RunTerraformCommand(terragruntOptions, terragruntOptions.TerraformCliArgs...)
}

// Returns the empty if the supplied command is not an extra command, otherwise, returns the command name
// to execute and the default arguments
func getExtraCommand(terragruntOptions *options.TerragruntOptions, config *config.TerragruntConfig) (string, string, []string) {
	cmd := terragruntOptions.TerraformCliArgs[0]
	for _, commands := range config.ExtraCommands {
		if len(commands.OS) > 0 && !util.ListContainsElement(commands.OS, runtime.GOOS) {
			continue
		}
		if len(commands.Commands) == 0 {
			commands.Commands = append(commands.Commands, commands.Name)
		}

		if commands.Name == cmd && !util.ListContainsElement(commands.Commands, cmd) {
			// The named command is not in the list of commands but match the commands name, in that case,
			// we consider that the name acts as an alias for the first command)
			cmd = commands.Commands[0]
		}

		if util.ListContainsElement(commands.Aliases, cmd) {
			// The named command is in the list of aliases, so we map it to the first command
			cmd = commands.Commands[0]
		}

		if util.ListContainsElement(commands.Commands, cmd) {
			var behaveAs string

			if commands.UseState == nil || *commands.UseState {
				// We simulate that the extra command acts as the plan command to init the state file
				// and get the modules
				behaveAs = "plan"
			}

			if commands.ActAs != "" {
				// The command must act as another command for extra argument validation
				terragruntOptions.TerraformCliArgs[0] = commands.ActAs
			}
			return cmd, behaveAs, commands.Arguments
		}
	}
	return "", "", nil
}

// Returns true if the command the user wants to execute is supposed to affect multiple Terraform modules, such as the
// apply-all or destroy-all command.
func isMultiModuleCommand(command string) bool {
	return strings.HasSuffix(command, MULTI_MODULE_SUFFIX)
}

// Execute a command that affects multiple Terraform modules, such as the apply-all or destroy-all command.
func runMultiModuleCommand(command string, terragruntOptions *options.TerragruntOptions) error {
	realCommand := strings.TrimSuffix(command, MULTI_MODULE_SUFFIX)
	if strings.HasPrefix(command, "plan-") {
		return planAll(realCommand, terragruntOptions)
	} else if strings.HasPrefix(command, "apply-") {
		return applyAll(realCommand, terragruntOptions)
	} else if strings.HasPrefix(command, "destroy-") {
		return destroyAll(realCommand, terragruntOptions)
	} else if strings.HasPrefix(command, "output-") {
		return outputAll(realCommand, terragruntOptions)
	} else {
		if strings.HasSuffix(command, MULTI_MODULE_SUFFIX) {
			return runAll(realCommand, terragruntOptions)
		} else {
			return errors.WithStackTrace(UnrecognizedCommand(command))
		}
	}
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
	return stack.RunAll([]string{command}, terragruntOptions, false)
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
		return stack.RunAll([]string{command, "-input=false"}, terragruntOptions, true)
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
		return stack.RunAll([]string{command, "-force", "-input=false"}, terragruntOptions, true)
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
