package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

		if importer.Prefix == nil {
			prefix := importer.Name + "_"
			importer.Prefix = &prefix
		}

		if importer.Required == nil {
			def := true
			importer.Required = &def
		}

		var sourceFolder string
		if importer.Source != "" {
			var err error
			sourceFolder, err = util.GetSource(importer.Source, terragruntOptions.Logger)
			if err != nil {
				return err
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

		var sourceFiles []string
		for _, pattern := range importer.Files {
			if sourceFolder != "" {
				pattern = filepath.Join(sourceFolder, pattern)
			} else if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(terragruntOptions.WorkingDir, pattern)
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
				terragruntOptions.Logger.Infof("%s: Copy %s to %s", importer.Name, strings.Join(fileBases, ", "), folderName)
			} else if *importer.Required {
				return fmt.Errorf("Unable to import required file %s", pattern)
			}
			sourceFiles = append(sourceFiles, files...)
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

// Execute the hooks. If OS is specified and the current OS is not listed, the command is ignored
func runHooks(terragruntOptions *options.TerragruntOptions, hooks []config.Hook) error {
	cmd := firstArg(terragruntOptions.TerraformCliArgs)
	for _, hook := range hooks {
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

func getModulesFolders(terragruntOptions *options.TerragruntOptions) ([]string, error) {
	modules, _ := filepath.Glob(filepath.Join(terragruntOptions.WorkingDir, ".terraform", "modules", "*"))
	folders := make(map[string]int)
	for _, module := range modules {
		stat, err := os.Stat(module)
		if err != nil {
			return nil, err
		}
		if !stat.IsDir() {
			terragruntOptions.Logger.Warning("Unexpected file in .terraform/modules:", module)
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
