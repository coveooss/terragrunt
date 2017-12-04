package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
)

func importFiles(terragruntOptions *options.TerragruntOptions, importers []config.ImportConfig, targetFolder string, isModule bool) error {
	var folderName string
	if !isModule {
		err := os.MkdirAll(targetFolder, 0755)
		if err != nil {
			return err
		}
		folderName = "temporary folder"
	} else {
		folderName = filepath.Base(targetFolder)
	}

	for _, importer := range importers {
		if len(importer.OS) > 0 && !util.ListContainsElement(importer.OS, runtime.GOOS) {
			terragruntOptions.Logger.Debugf("Importer %s skipped, executed only on %v", importer.Name, importer.OS)
			continue
		}

		if isModule && !importer.ImportIntoModules {
			continue
		}

		if importer.Required == nil {
			def := true
			importer.Required = &def
		}

		if importer.Prefix == nil {
			def := ""
			importer.Prefix = &def
		}

		var sourceFolder string
		if importer.Source != "" {
			var err error
			sourceFolder, err = util.GetSource(importer.Source, terragruntOptions.Logger)
			if err != nil {
				if *importer.Required {
					return err
				}
				terragruntOptions.Logger.Warningf("%s: %s doesn't exist", importer.Name, importer.Source)
			}
		}

		// Check if the importer has a specific target folder
		importerTarget := targetFolder
		if importer.Target != "" {
			folderName = importer.Target
			if filepath.IsAbs(importer.Target) {
				importerTarget = importer.Target
			} else {
				importerTarget = filepath.Join(targetFolder, importer.Target)
			}
			err := os.MkdirAll(importerTarget, 0755)
			if err != nil {
				return err
			}
		}

		// Local copy function used by both type of file copy
		copy := func(source, target string) error {
			target = filepath.Join(importerTarget, target)
			if err := util.CopyFile(source, target); err != nil {
				return err
			}
			if importer.FileMode != nil {
				return os.Chmod(target, os.FileMode(*importer.FileMode))
			}
			return nil
		}

		ensureIsFile := func(file string) error {
			if stat, err := os.Stat(file); err != nil {
				return fmt.Errorf("Error %v while processing %s: %s", err, importer.Name, file)
			} else if stat.IsDir() {
				return fmt.Errorf("Folder ignored %s: %s", importer.Name, file)
			}
			return nil
		}

		var sourceFiles []string
		if len(importer.Files) == 0 {
			importer.Files = []string{"*"}
		}

		var filePatterns, pathPatterns []string
		for _, pattern := range importer.Files {
			if filepath.IsAbs(pattern) {
				files, err := filepath.Glob(pattern)
				if err != nil {
					return fmt.Errorf("Invalid pattern %s", filepath.Base(pattern))
				}
				if *importer.Required && len(files) == 0 {
					return fmt.Errorf("Unable to import required file %s (%s)", importer.Name, pattern)
				}

				for _, file := range files {
					if err := ensureIsFile(file); err == nil {
						sourceFiles = append(sourceFiles, file)
					}
				}
			} else if strings.ContainsAny(pattern, "*[]?") {
				if strings.Contains(pattern, string(filepath.Separator)) {
					pathPatterns = append(pathPatterns, pattern)
				} else {
					filePatterns = append(filePatterns, pattern)
				}
			}
		}

		_ = filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
			if ensureIsFile(path) != nil || strings.HasSuffix(path, aws_helper.CacheFile) {
				return nil
			}
			relName := strings.TrimPrefix(path, sourceFolder)[1:]
			for _, pattern := range filePatterns {
				if match, _ := filepath.Match(pattern, filepath.Base(relName)); match {
					sourceFiles = append(sourceFiles, path)
				}
			}
			for _, pattern := range pathPatterns {
				if match, _ := filepath.Match(pattern, relName); match {
					sourceFiles = append(sourceFiles, path)
				}
			}
			return nil
		})

		if *importer.Required && len(sourceFiles) == 0 {
			return fmt.Errorf("Unable to import required file %s (%s)", importer.Name, strings.Join(importer.Files, ", "))
		}

		for _, source := range sourceFiles {
			if util.FileExists(source) {
				if err := copy(source, *importer.Prefix+filepath.Base(source)); err != nil {
					return err
				}
			} else if *importer.Required {
				return fmt.Errorf("Unable to import required file %s", source)
			} else if !isModule {
				terragruntOptions.Logger.Debugf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}

		for _, source := range importer.CopyAndRenameFiles {
			if util.FileExists(source.Source) {
				terragruntOptions.Logger.Infof("Copy file %s to %s/%v", filepath.Base(source.Source), folderName, source.Target)
				if err := copy(source.Source, source.Target); err != nil {
					return err
				}
			} else if *importer.Required {
				return fmt.Errorf("Unable to import required file %s", source.Source)
			} else if !isModule {
				terragruntOptions.Logger.Debugf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}
	}
	return nil
}

// Used to filter the hook on supplied criteria
type hookFilter func(config.Hook) bool

// Execute the hooks. If OS is specified and the current OS is not listed, the command is ignored
func runHooks(terragruntOptions *options.TerragruntOptions, hooks []config.Hook, filter hookFilter) error {
	cmd := terragruntOptions.Env["TERRAGRUNT_COMMAND"]

	sort.Slice(hooks, func(i, j int) bool { return hooks[i].Order < hooks[j].Order || i < j })

	for _, hook := range hooks {
		if filter != nil && !filter(hook) {
			continue
		}

		if len(hook.OnCommands) > 0 && !util.ListContainsElement(hook.OnCommands, cmd) {
			// The current command is not in the list of command on which the hook should be applied
			continue
		}
		if len(hook.OS) > 0 && !util.ListContainsElement(hook.OS, runtime.GOOS) {
			terragruntOptions.Logger.Debugf("Hook %s skipped, executed only on %v", hook.Name, hook.OS)
			continue
		}
		hook.Command = strings.TrimSpace(hook.Command)
		if len(hook.Command) == 0 {
			terragruntOptions.Logger.Debugf("Hook %s skipped, no command to execute", hook.Name)
			continue
		}
		cmd := shell.RunShellCommand
		if hook.ExpandArgs {
			cmd = shell.RunShellCommandExpandArgs
		}
		if err := cmd(terragruntOptions, hook.Command, hook.Arguments...); err != nil && !hook.IgnoreError {
			return fmt.Errorf("%v while running command %s: %s %s", err, hook.Name, hook.Command, strings.Join(hook.Arguments, " "))
		}
	}
	return nil
}

func importDefaultVariables(terragruntOptions *options.TerragruntOptions, folder string) error {
	// Retrieve the default variables from the terraform files
	variables, err := util.LoadDefaultValues(folder)
	if err != nil {
		return err
	}
	for key, value := range variables {
		terragruntOptions.Variables.SetValue(key, value, options.Default)
	}
	return nil
}

func getModulesFolders(terragruntOptions *options.TerragruntOptions) ([]string, error) {
	modules, _ := filepath.Glob(filepath.Join(terragruntOptions.WorkingDir, ".terraform", "modules", "*"))
	folders := make(map[string]int)
	for _, module := range modules {
		stat, err := os.Stat(module)
		if err != nil {
			return nil, err
		}
		if !stat.IsDir() {
			continue
		}

		stat, _ = os.Lstat(module)
		if !stat.IsDir() {
			link, err := os.Readlink(module)
			if err != nil {
				return nil, err
			}
			module = link
		}
		folders[module] = folders[module] + 1
	}

	keys := make([]string, 0, len(folders))
	for key := range folders {
		keys = append(keys, key)
	}
	return keys, nil
}

func setRoleEnvironmentVariables(terragruntOptions *options.TerragruntOptions, roleArn string) error {
	roleVars, err := aws_helper.AssumeRoleEnvironmentVariables(roleArn, "terragrunt")
	if err != nil {
		return err
	}

	for key, value := range roleVars {
		terragruntOptions.Env[key] = value
	}
	return nil
}

func getActualCommand(terragruntOptions *options.TerragruntOptions, config *config.TerragruntConfig) actualCommand {
	cmd := terragruntOptions.TerraformCliArgs[0]
	for _, extraCommand := range config.ExtraCommands {
		if len(extraCommand.OS) > 0 && !util.ListContainsElement(extraCommand.OS, runtime.GOOS) {
			continue
		}

		if extraCommand.Command == "" {
			// If there is no explicit command for this extra command, we consider it as to be either
			// the first command listed in Commands or the name of the extra command
			if len(extraCommand.Commands) != 0 {
				extraCommand.Command = extraCommand.Commands[0]
			} else {
				extraCommand.Command = extraCommand.Name
			}
		}

		cmd = resolveAlias(cmd, &extraCommand, terragruntOptions)

		if cmd == extraCommand.Command || util.ListContainsElement(extraCommand.Commands, cmd) {
			var behaveAs string

			if !util.ListContainsElement(extraCommand.Commands, cmd) {
				// If the command is not in the allowed list of commands, we then default it to the default command
				cmd = extraCommand.Command
			}

			if extraCommand.ExpandArgs == nil {
				// We default the ExpandArgs to true if not set
				expand := true
				extraCommand.ExpandArgs = &expand
			}
			if extraCommand.ActAs != "" {
				// The command must act as another command for extra argument validation
				terragruntOptions.TerraformCliArgs[0] = extraCommand.ActAs
			} else {
				extraCommand.ActAs = cmd
				if extraCommand.UseState == nil || *extraCommand.UseState {
					// We simulate that the extra command acts as the plan command to init the state file
					// and get the modules
					behaveAs = "plan"
				}
			}

			return actualCommand{cmd, behaveAs, &extraCommand}
		}
	}
	return actualCommand{cmd, "", nil}
}

func resolveAlias(cmd string, extra *config.ExtraCommand, terragruntOptions *options.TerragruntOptions) string {
	for _, alias := range extra.Aliases {
		split := strings.SplitN(alias, "=", 2)
		if cmd != split[0] {
			continue
		}

		if len(split) == 1 {
			return extra.Command
		}

		cmd = split[1]
		if strings.ContainsAny(split[1], " |,&$") {
			cmd = "bash"

			if !util.ListContainsElement(extra.Commands, cmd) {
				extra.Commands = append(extra.Commands, cmd)
			}

			var args string
			for _, arg := range append(extra.Arguments, terragruntOptions.TerraformCliArgs[1:]...) {
				if !strings.Contains(arg, " \t") {
					args += " " + arg
				} else {
					args += fmt.Sprintf(` "%s"`, arg)
				}
			}

			script := split[1]
			if strings.Contains(script, " $*") {
				script = strings.Replace(script, " $*", args, -1)
			} else if !strings.Contains(script, "|") {
				script += args
			}

			extra.Arguments = []string{"-c", script}
			terragruntOptions.TerraformCliArgs = terragruntOptions.TerraformCliArgs[:1]
		}
	}
	return cmd
}

type actualCommand struct {
	Command  string
	BehaveAs string
	Extra    *config.ExtraCommand
}
