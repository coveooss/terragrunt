package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/awshelper"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/remote"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/terraform/configs"
	"github.com/rs/xid"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Constant used to define the command line options
const (
	optTerragruntConfig                 = "terragrunt-config"
	optTerragruntTFPath                 = "terragrunt-tfpath"
	optApprovalHandler                  = "terragrunt-approval"
	optNonInteractive                   = "terragrunt-non-interactive"
	optWorkingDir                       = "terragrunt-working-dir"
	optTerragruntSource                 = "terragrunt-source"
	optTerragruntSourceUpdate           = "terragrunt-source-update"
	optTerragruntIgnoreDependencyErrors = "terragrunt-ignore-dependency-errors"
	optLoggingLevel                     = "terragrunt-logging-level"
	optLoggingFileDir                   = "terragrunt-logging-file-dir"
	optLoggingFileLevel                 = "terragrunt-logging-file-level"
	optFlushDelay                       = "terragrunt-flush-delay"
	optNbWorkers                        = "terragrunt-workers"
	optAWSProfile                       = "profile"
	optApplyTemplate                    = "terragrunt-apply-template"
	optTemplatePatterns                 = "terragrunt-template-patterns"
	optBootConfigs                      = "terragrunt-boot-configs"
	optPreBootConfigs                   = "terragrunt-pre-boot-configs"
)

var allTerragruntBooleanOpts = []string{optNonInteractive, optTerragruntSourceUpdate, optTerragruntIgnoreDependencyErrors, optApplyTemplate}
var allTerragruntStringOpts = []string{optTerragruntConfig, optTerragruntTFPath, optWorkingDir, optTerragruntSource, optLoggingLevel, optAWSProfile, optApprovalHandler, optFlushDelay, optNbWorkers, optTemplatePatterns, optBootConfigs, optPreBootConfigs, optLoggingFileDir, optLoggingFileLevel}

const multiModuleSuffix = "-all"
const cmdInit = "init"

var terraformCommandsThatUseState = []string{
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

var terraformCommandsWithSubCommand = []string{
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
   <command> --help | -h               Print the command detailed help 

   get-doc [options...] [filters...] Print the documentation of all extra_arguments, import_files, pre_hook, post_hook and extra_command.
   get-versions                      Get all versions of underlying tools (including extra_command).
   get-stack [options]               Get the list of stack to execute sorted by dependency order.

   -all operations:
   plan-all                          Display the plans of a 'stack' by running 'terragrunt plan' in each subfolder (with a summary at the end).
   apply-all                         Apply a 'stack' by running 'terragrunt apply' in each subfolder.
   output-all                        Display the outputs of a 'stack' by running 'terragrunt output' in each subfolder (no error if a subfolder doesn't have outputs).
   destroy-all                       Destroy a 'stack' by running 'terragrunt destroy' in each subfolder in reverse dependency order.
   *-all                             In fact, the -all could be applied on any terraform or custom commands (that's cool).

   terraform commands:
   *             Terragrunt forwards all other commands directly to Terraform

GLOBAL OPTIONS:
   terragrunt-config                    Path to the Terragrunt config file. Default is terragrunt.hcl.
   terragrunt-tfpath                    Path to the Terraform binary. Default is terraform (on PATH).
   terragrunt-non-interactive           Assume "yes" for all prompts.
   terragrunt-working-dir               The path to the Terraform templates. Default is current directory.
   terragrunt-source                    Download Terraform configurations from the specified source into a temporary folder, and run Terraform in that temporary folder.
   terragrunt-source-update             Delete the contents of the temporary folder to clear out any old, cached source code before downloading new source code into it.
   terragrunt-ignore-dependency-errors  *-all commands continue processing components even if a dependency fails.
   terragrunt-logging-level             PANIC(0), FATAL (1), ERROR (2), WARNING (3), INFO (4), DEBUG (5), TRACE (6).
   terragrunt-logging-file-dir          Used to configure the directory where (verbose) file logs will be saved
   terragrunt-logging-file-level        Used to configure the logging level in files
   terragrunt-approval                  Program to use for approval. {val} will be replaced by the current terragrunt output. Ex: approval.py --value {val}
   terragrunt-flush-delay               Maximum delay on -all commands before printing out traces (INFO) indicating that the process is still alive (default 60s).
   terragrunt-workers                   Number of concurrent workers (default 10).
   profile                              Specify an AWS profile to use.

ENVIRONMENT VARIABLES:
   The following environment variables could be set to avoid specifying parameters on command line:
	  TERRAGRUNT_CONFIG, TERRAGRUNT_TFPATH, TERRAGRUNT_SOURCE, TERRAGRUNT_LOGGING_LEVEL, TERRAGRUNT_FLUSH_DELAY, TERRAGRUNT_WORKERS, TERRAGRUNT_LOGGING_FILE_DIR, TERRAGRUNT_LOGGING_FILE_LEVEL
	  
   TERRAGRUNT_DEBUG  If set, this enable detailed stack trace in case of application crash
   TERRAGRUNT_CACHE  If set, it defines the root folder used to store temporary files

   The following AWS variables are used to determine the default configuration.
      AWS_PROFILE, AWS_REGION, AWS_ACCESS_KEY_ID

VERSION:
   {{.Version}}{{if len .Authors}}

AUTHOR(S):
   {{range .Authors}}{{.}}{{end}}
   {{end}}
`

var moduleRegex = regexp.MustCompile(`module[[:blank:]]+".+"`)

// This uses the constraint syntax from https://github.com/hashicorp/go-version
const defaultTerraformVersionConstaint = ">= v0.9.3"

const terraformExtensionGlob = "*.tf"

var terragruntVersion string
var terragruntRunID string
var terraformVersion string

// CreateTerragruntCli creates the Terragrunt CLI App.
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

	if cliContext.Args().First() == "0.12upgrade" {
		return migrate(cliContext)
	}

	terragruntRunID = fmt.Sprint(xid.New())

	os.Setenv(options.EnvCacheFolder, util.GetTempDownloadFolder("terragrunt-cache"))
	os.Setenv(options.EnvArgs, strings.Join(os.Args, " "))
	os.Setenv(options.EnvRunID, terragruntRunID)

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
		terragruntOptions.Println(title("TERRAFORM\n"))
		shell.NewTFCmd(terragruntOptions).Args("--help").Run()
		return nil
	}

	// If AWS is configured, we init the session to ensure that proper environment variables are set
	if terragruntOptions.AwsProfile != "" || os.Getenv("AWS_PROFILE") != "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		_, err := awshelper.InitAwsSession(terragruntOptions.AwsProfile)
		if err != nil {
			return err
		}
	}

	if err := CheckTerraformVersion(defaultTerraformVersionConstaint, terragruntOptions); err != nil {
		return err
	}

	return runCommand(cliContext.Args().First(), terragruntOptions)
}

// runCommand runs one or many terraform commands based on the type of
// terragrunt command
func runCommand(command string, terragruntOptions *options.TerragruntOptions) (finalEff error) {
	if err := setRoleEnvironmentVariables(terragruntOptions, "", nil); err != nil {
		return err
	}
	isMultiModules := command == getStackCommand || strings.HasSuffix(command, multiModuleSuffix)
	terragruntOptions.Context = map[string]interface{}{
		"Version":              terragruntVersion,
		"AwsProfile":           terragruntOptions.AwsProfile,
		"DownloadDir":          terragruntOptions.DownloadDir,
		"LoggingLevel":         terragruntOptions.Logger.GetHookLevel(""),
		"LoggingLevelName":     terragruntOptions.Logger.GetHookLevel("").String(),
		"NbWorkers":            terragruntOptions.NbWorkers,
		"SourceUpdate":         terragruntOptions.SourceUpdate,
		"TerraformCliArgs":     terragruntOptions.TerraformCliArgs,
		"TerraformPath":        terragruntOptions.TerraformPath,
		"TerragruntConfigPath": terragruntOptions.TerragruntConfigPath,
		"WorkingDir":           terragruntOptions.WorkingDir,
		"RunID":                os.Getenv(options.EnvRunID),
		"Command":              command,
		"IsMultiModules":       isMultiModules,
	}

	if isMultiModules {
		return runMultiModuleCommand(command, terragruntOptions)
	}
	return runTerragrunt(terragruntOptions)
}

var runHandler func(*options.TerragruntOptions, *config.TerragruntConfig) error

// Run Terragrunt with the given options and CLI args. This will forward all the args directly to Terraform, enforcing
// best practices along the way.
func runTerragrunt(terragruntOptions *options.TerragruntOptions) (finalStatus error) {
	defer func() {
		if _, hasStack := finalStatus.(*errors.Error); finalStatus != nil && !hasStack {
			finalStatus = errors.WithStackTrace(finalStatus)
		}
		terragruntOptions.CloseWriters()
	}()

	if util.FileExists(filepath.Join(terragruntOptions.WorkingDir, options.IgnoreFile)) {
		return fmt.Errorf("folder ignored because %s is present", options.IgnoreFile)
	}

	if terragruntOptions.NonInteractive && util.FileExists(filepath.Join(terragruntOptions.WorkingDir, options.IgnoreFileNonInteractive)) {
		return fmt.Errorf("folder ignored because %s is present", options.IgnoreFileNonInteractive)
	}

	conf, err := config.ReadTerragruntConfig(terragruntOptions)
	if err != nil {
		return err
	}

	sourceURL, hasSourceURL := getTerraformSourceURL(terragruntOptions, conf)
	if sourceURL == "" {
		sourceURL = terragruntOptions.WorkingDir
	}
	terragruntOptions.Env[options.EnvLaunchFolder] = terragruntOptions.WorkingDir

	if terragruntOptions.Env[options.EnvSourceFolder], err = util.CanonicalPath(sourceURL, ""); err != nil {
		return err
	}

	// If runHandler has been specified, we bypass the planned execution and defer the control to the handler
	if runHandler != nil {
		return runHandler(terragruntOptions, conf)
	}

	// Check if the current command is an extra command
	actualCommand := conf.ExtraCommands.ActualCommand(terragruntOptions.TerraformCliArgs[0])
	ignoreError := actualCommand.Extra != nil && actualCommand.Extra.IgnoreError
	terragruntOptions.Env[options.EnvCommand] = terragruntOptions.TerraformCliArgs[0]

	stopOnError := func(err error) bool {
		if err == nil {
			return false
		}

		if _, planStatusError := err.(errors.PlanWithChanges); planStatusError {
			finalStatus = errors.WithStackTrace(err)
			return false
		}

		if !ignoreError {
			finalStatus = errors.WithStackTrace(err)
			return true
		}
		terragruntOptions.Logger.Error("Error, but continuing execution because ignore_error is set")
		terragruntOptions.Logger.Debugf("%v", err)
		if finalStatus == nil {
			finalStatus = errors.WithStackTrace(err)
		}
		return false
	}

	if conf.UniquenessCriteria != nil {
		// If uniqueness_criteria has been defined, we set it in the options to ensure that
		// we use distinct folder based on this criteria
		terragruntOptions.UniquenessCriteria = *conf.UniquenessCriteria
	}

	// Copy the deployment files to the working directory
	terraformSource, err := processTerraformSource(sourceURL, terragruntOptions)
	if stopOnError(err) {
		return err
	}

	useTempFolder := hasSourceURL || len(conf.ImportFiles)+len(conf.ExportVariablesConfigs) > 0
	if useTempFolder {
		// If there are import files, we force the usage of a temp directory.
		if err = downloadTerraformSource(terraformSource, terragruntOptions); err != nil {
			absSourceURL, _ := filepath.Abs(sourceURL)
			pathMsg := color.WhiteString("\nVerify that the following path exists:\n  Given:    %s\n  Absolute: %s", sourceURL, absSourceURL)
			return fmt.Errorf("Could not copy your source folder to a temporary location.\n%w\n%s", err, pathMsg)
		}
	}

	if err = conf.ImportVariables.Import(); stopOnError(err) {
		return
	}

	// Applying the extra arguments
	if len(conf.ExtraArgs) > 0 {
		commandLength := 1
		if util.ListContainsElement(terraformCommandsWithSubCommand, terragruntOptions.TerraformCliArgs[0]) {
			commandLength = 2
		}

		// Options must be inserted after command but before the other args command is either 1 word or 2 words
		var args []string
		args = append(args, terragruntOptions.TerraformCliArgs[:commandLength]...)
		extraArgs, err := conf.ExtraArguments(sourceURL)
		if stopOnError(err) {
			return
		}

		args = append(args, extraArgs...)
		if commandLength <= len(terragruntOptions.TerraformCliArgs) {
			args = append(args, terragruntOptions.TerraformCliArgs[commandLength:]...)
		}
		terragruntOptions.TerraformCliArgs = args
	}

	// Determinate if the project should be ignored
	if !conf.RunConditions.ShouldRun() {
		return nil
	}

	// Executing the pre-hook commands that should be ran before the ImportFiles
	if _, err = conf.PreHooks.Filter(config.BeforeImports).Run(err); stopOnError(err) {
		return
	}

	// Import the required files in the temporary folder and copy the temporary imported file in the
	// working folder. We did not put them directly into the folder because terraform init would complain
	// if there are already terraform files in the target folder
	if err := conf.ImportFiles.Run(err); stopOnError(err) {
		return
	}

	// Retrieve the default variables from the terraform files
	if err = importDefaultVariables(terragruntOptions, terragruntOptions.WorkingDir); stopOnError(err) {
		return
	}

	if actualCommand.Command == "get-versions" {
		PrintVersions(terragruntOptions, conf)
		return
	}

	if actualCommand.Command == "get-doc" {
		PrintDoc(terragruntOptions, conf)
		return
	}

	// Check if we must configure environment variables to assume a distinct role when applying external commands.
	if conf.AssumeRole != nil {
		var roleAssumed bool
		for i := range conf.AssumeRole {
			role := strings.TrimSpace(conf.AssumeRole[i])
			if role == "" {
				listOfRoles := strings.Join(conf.AssumeRole[:i], ", ")
				if listOfRoles != "" {
					listOfRoles = " from " + listOfRoles
				}
				terragruntOptions.Logger.Warningf("Not assuming any role%s, continuing with the current user credentials", listOfRoles)
				roleAssumed = true
				break
			}
			if err := setRoleEnvironmentVariables(terragruntOptions, role, conf.AssumeRoleDurationHours); err == nil {
				terragruntOptions.Logger.Debug("Assumed role ", role)
				roleAssumed = true
				break
			}
		}
		if !roleAssumed {
			return fmt.Errorf("unable to assume any of the roles: %s", strings.Join(conf.AssumeRole, " "))
		}
	}

	if actualCommand.Extra != nil {
		terragruntOptions.Env[options.EnvExtraCommand] = actualCommand.Command
	}
	terragruntOptions.Env[options.EnvVersion] = terragruntVersion
	terragruntOptions.Env[options.EnvTFVersion] = terraformVersion

	// Temporary make the command behave as another command to initialize the folder properly
	// (to be sure that the remote state file get initialized)
	if actualCommand.BehaveAs != "" {
		terragruntOptions.TerraformCliArgs[0] = actualCommand.BehaveAs
	}

	terraformFiles := utils.MustFindFiles(terragruntOptions.WorkingDir, true, false, "*.tf")
	foldersWithTerraformFiles := []string{}
	for _, file := range terraformFiles {
		// All folders except `.terraform`
		if dir := filepath.Dir(file); !util.ListContainsElement(foldersWithTerraformFiles, dir) && !strings.HasPrefix(dir, path.Join(terragruntOptions.WorkingDir, ".terraform")) {
			foldersWithTerraformFiles = append(foldersWithTerraformFiles, dir)
		}
	}

	if err := conf.ImportFiles.Run(err, foldersWithTerraformFiles...); stopOnError(err) {
		return
	}

	// Run Gotemplate
	if err == nil && useTempFolder && terragruntOptions.ApplyTemplate {
		template.TemplateLog.SetDefaultConsoleHookLevel(terragruntOptions.Logger.GetLevel())
		var t *template.Template
		if t, err = template.NewTemplate(terragruntOptions.WorkingDir, terragruntOptions.GetContext(), "", nil); stopOnError(err) {
			return
		}
		t.SetOption(template.Overwrite, true)
		patterns := append(terragruntOptions.TemplateAdditionalPatterns, "*.tf", "*.tf.gt", "*.tf.template")
		files := utils.MustFindFiles(terragruntOptions.WorkingDir, true, false, patterns...)
		filterPath := func(s string) string { return strings.Replace(s, terragruntOptions.WorkingDir+"/", "", -1) }
		files = util.FilterList(files, func(item string) bool {
			return !strings.HasPrefix(filterPath(item), ".terraform") // Do not run on cached modules
		})
		modifiedFiles, err := t.ProcessTemplates("", "", files...)
		if err != nil {
			err = fmt.Errorf("error(s) while applying go template\n%s", filterPath(err.Error()))
			if stopOnError(err) {
				return
			}
		}
		if len(modifiedFiles) > 0 {
			terragruntOptions.Logger.Debugf("File(s) modified by go template: %s", filterPath(strings.Join(modifiedFiles, ", ")))
		}
	}

	// Export Terragrunt variables to the paths defined in export_variables blocks
	for _, folder := range foldersWithTerraformFiles {
		var existingVariables map[string]*configs.Variable
		_, existingVariables, err = util.LoadDefaultValues(folder)
		if stopOnError(err) {
			return err
		}
		if err = conf.ExportVariables(existingVariables, folder); stopOnError(err) {
			return
		}
	}

	if err := downloadModules(terragruntOptions); stopOnError(err) {
		return
	}

	// If there is no terraform file in the folder, we skip the command
	tfFiles, err := utils.FindFiles(terragruntOptions.WorkingDir, false, false, "*.tf", "*.tf.json")
	if stopOnError(err) {
		return
	}
	if len(tfFiles) == 0 {
		terragruntOptions.Logger.Warning("No terraform file found, skipping folder")
		return nil
	}

	// Set the temporary script folder as the first item of the PATH
	terragruntOptions.Env["PATH"] = fmt.Sprintf("%s%c%s", filepath.Join(terraformSource.WorkingDir, config.TerragruntScriptFolder), filepath.ListSeparator, terragruntOptions.Env["PATH"])

	// Executing the pre-hook commands that should be ran before init state if there are
	if _, err = conf.PreHooks.Filter(config.BeforeInitState).Run(err); stopOnError(err) {
		return
	}

	// Configure remote state if required
	if conf.RemoteState != nil {
		if err := configureRemoteState(conf.RemoteState, terragruntOptions); stopOnError(err) {
			return
		}
	}

	// Executing the pre-hook that should be ran after init state if there are
	if _, err = conf.PreHooks.Filter(config.AfterInitState).Run(err); stopOnError(err) {
		return
	}

	defer func() {
		// If there is an error but it is in fact a plan status, we run the post hooks normally
		_, planStatusError := err.(errors.PlanWithChanges)

		// Executing the post-hook commands if there are and there is no error
		status := err
		if planStatusError {
			status = nil
		}
		if _, errHook := conf.PostHooks.Run(status); stopOnError(errHook) {
			return
		}
	}()

	// Run an init in case there are new modules or plugins to import
	shell.NewTFCmd(terragruntOptions).Args([]string{"init", "--backend=false"}...).WithRetries(3).Output()

	isApply := actualCommand.Command == "apply" || (actualCommand.Extra != nil && actualCommand.Extra.ActAs == "apply")
	if terragruntOptions.NonInteractive && isApply && !util.ListContainsElement(terragruntOptions.TerraformCliArgs, "-auto-approve") {
		terragruntOptions.TerraformCliArgs = append(terragruntOptions.TerraformCliArgs, "-auto-approve")
	}

	var cmd *shell.CommandContext

	if actualCommand.Extra != nil {
		// The command is not a native terraform command
		expandArgs := *actualCommand.Extra.ExpandArgs
		command := actualCommand.Command
		args := append(actualCommand.Extra.Arguments, terragruntOptions.TerraformCliArgs[1:]...)

		cmd = shell.NewCmd(terragruntOptions, command).Args(args...)
		if actualCommand.Extra.ShellCommand {
			// We must not redirect the stderr on shell command, doing so, remove the prompt
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		}

		if expandArgs {
			cmd = cmd.ExpandArgs()
		}

		actualCommand.Command = actualCommand.Extra.ActAs
	} else {
		// If the command is 'init', stop here. That's because ConfigureRemoteState above will have already called
		// terraform init if it was necessary, and the below RunTerraformCommand would end up calling init without
		// the correct remote state arguments, which is confusing.
		if terragruntOptions.TerraformCliArgs[0] == cmdInit {
			terragruntOptions.Logger.Warning("Running 'init' manually is not necessary: Terragrunt will call it automatically when needed before running other Terraform commands")
			return nil
		}

		// We restore back the name of the command since it may have been temporary changed to support state file initialization and get modules
		terragruntOptions.TerraformCliArgs[0] = actualCommand.Command

		cmd = shell.NewTFCmd(terragruntOptions).Args(terragruntOptions.TerraformCliArgs...)
	}
	if shouldBeApproved, approvalConfig := conf.ApprovalConfig.ShouldBeApproved(actualCommand.Command); shouldBeApproved {
		cmd = cmd.Expect(approvalConfig.ExpectStatements, approvalConfig.CompletedStatements)
	}
	err = shell.FilterPlanError(cmd.Run(), actualCommand.Command)

	exitCode, errCode := shell.GetExitCode(err)
	if errCode != nil {
		exitCode = -1
	}
	terragruntOptions.SetStatus(exitCode, err)

	if stopOnError(err) {
		return
	}
	return
}

// Execute a command that affects multiple Terraform modules, such as the apply-all or destroy-all command.
func runMultiModuleCommand(command string, terragruntOptions *options.TerragruntOptions) error {
	realCommand := strings.TrimSuffix(command, multiModuleSuffix)
	terragruntOptions.Context["Command"] = realCommand

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
	} else if strings.HasSuffix(command, multiModuleSuffix) {
		return runAll(realCommand, terragruntOptions)
	}
	return errors.WithStackTrace(unrecognizedCommand(command))
}

// A quick sanity check that calls `terraform get` to download modules, if they aren't already downloaded.
func downloadModules(terragruntOptions *options.TerragruntOptions) error {
	command := util.IndexOrDefault(terragruntOptions.TerraformCliArgs, 0, "")
	if util.ListContainsElement(terraformCommandsThatUseState, command) {
		shouldDownload, err := shouldDownloadModules(terragruntOptions)
		if err != nil {
			return err
		}
		if shouldDownload {
			return shell.NewTFCmd(terragruntOptions).Args("get", "-update").LogOutput(logrus.DebugLevel)
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

	return util.Grep(moduleRegex, fmt.Sprintf("%s/%s", terragruntOptions.WorkingDir, terraformExtensionGlob))
}

// If the user entered a Terraform command that uses state (e.g. plan, apply), make sure remote state is configured
// before running the command.
func configureRemoteState(remoteState *remote.State, terragruntOptions *options.TerragruntOptions) error {
	// We only configure remote state for the commands that use the tfstate files. We do not configure it for
	// commands such as "get" or "version".
	if util.ListContainsElement(terraformCommandsThatUseState, util.IndexOrDefault(terragruntOptions.TerraformCliArgs, 0, "")) {
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

	terragruntOptions.Logger.Debug(stack)
	return stack.RunAll([]string{command}, terragruntOptions, configstack.NormalOrder)
}

// planAll prints the plans from all configuration in a stack, in the order
// specified in the terraform_remote_state dependencies
func planAll(command string, terragruntOptions *options.TerragruntOptions) error {
	stack, err := configstack.FindStackInSubfolders(terragruntOptions)
	if err != nil {
		return err
	}

	terragruntOptions.Logger.Debug(stack.String())
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

	terragruntOptions.Logger.Debug(stack)
	return stack.Output(command, terragruntOptions)
}

// Custom error types

type unrecognizedCommand string

func (commandName unrecognizedCommand) Error() string {
	return fmt.Sprintf("Unrecognized command: %s", string(commandName))
}
