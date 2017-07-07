package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
)

func importFiles(terragruntOptions *options.TerragruntOptions, importers []config.ImportConfig, targetFolder string, isModule bool) (err error) {
	var folderName string
	if !isModule {
		os.MkdirAll(targetFolder, 0755)
		folderName = "temporary folder"
	} else {
		folderName = filepath.Base(targetFolder)
	}

	for _, importer := range importers {
		var sourceFolder string
		if isModule && !importer.ImportIntoModules {
			continue
		}

		if importer.Source != "" {
			sourceFolder, err = util.GetSource(importer.Source, terragruntOptions.TerraformPath)
			if err != nil {
				return err
			}
		}

		var sourceFiles []string
		for _, pattern := range importer.Files {
			if sourceFolder != "" {
				pattern = filepath.Join(sourceFolder, pattern)
			}
			files, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("Invalid pattern %s", filepath.Base(pattern))
			}

			if len(files) > 0 {
				fileBases := make([]string, len(files))
				for i, file := range files {
					fileBases[i] = filepath.Base(file)
				}
				terragruntOptions.Logger.Noticef("%s: Copy %s to %s", importer.Name, strings.Join(fileBases, ", "), folderName)
			}
			sourceFiles = append(sourceFiles, files...)
		}

		for _, source := range sourceFiles {
			if util.FileExists(source) {
				base := filepath.Base(source)
				target := filepath.Join(targetFolder, base)
				if err := util.CopyFile(source, target); err != nil {
					return err
				}
			} else if importer.Required {
				return fmt.Errorf("Unable to import required file %s", source)
			} else if !isModule {
				terragruntOptions.Logger.Warningf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}

		for _, source := range importer.CopyAndRenameFiles {
			if util.FileExists(source.Source) {
				target := filepath.Join(targetFolder, source.Target)
				terragruntOptions.Logger.Noticef("Copy file %s to %s/%v", filepath.Base(source.Source), folderName, source.Target)
				if err := util.CopyFile(source.Source, target); err != nil {
					return err
				}
			} else if importer.Required {
				return fmt.Errorf("Unable to import required file %s", source.Source)
			} else if !isModule {
				terragruntOptions.Logger.Warningf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}
	}
	return nil
}

// Execute the hooks. If OS is specified and the current OS is not listed, the command is ignored
func runHooks(terragruntOptions *options.TerragruntOptions, hooks []config.Hook) error {
	cmd := firstArg(terragruntOptions.TerraformCliArgs)
	for _, hook := range hooks {
		if len(hook.OnCommands) > 0 && !util.ListContainsElement(hook.OnCommands, cmd) {
			// The current command is not in the list of command on which the hook should be applied
			continue
		}
		if len(hook.OS) > 0 && !util.ListContainsElement(hook.OS, runtime.GOOS) {
			terragruntOptions.Logger.Infof("Hook %s skipped, executed only on %v", hook.Name, hook.OS)
			continue
		}
		hook.Command = strings.TrimSpace(hook.Command)
		if len(hook.Command) == 0 {
			terragruntOptions.Logger.Infof("Hook %s skipped, no command to execute", hook.Name)
			continue
		}
		cmd := shell.RunShellCommand
		if hook.ExpandArgs {
			cmd = shell.RunShellCommandExpandArgs
		}
		if err := cmd(terragruntOptions, hook.Command, hook.Arguments...); err != nil {
			return fmt.Errorf("%v while running command %s: %s %s", err, hook.Name, hook.Command, strings.Join(hook.Arguments, " "))
		}
	}
	return nil
}

func importVariables(terragruntOptions *options.TerragruntOptions, folder string) error {
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
