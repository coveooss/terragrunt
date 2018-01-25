package config

import (
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/coveo/gotemplate/utils"
)

// SubstituteAllVariables replace all remaining variables by the value
func (conf *TerragruntConfig) SubstituteAllVariables(terragruntOptions *options.TerragruntOptions, substituteFinal bool) {
	scriptFolder := filepath.Join(terragruntOptions.WorkingDir, TerragruntScriptFolder)
	substitute := func(value *string) *string {
		if value == nil {
			return nil
		}

		*value = SubstituteVars(*value, terragruntOptions)
		if substituteFinal {
			// We only substitute folders on the last substitute call
			*value = strings.Replace(*value, GET_TEMP_FOLDER, terragruntOptions.DownloadDir, -1)
			*value = strings.Replace(*value, GET_SCRIPT_FOLDER, scriptFolder, -1)
			*value = strings.TrimSpace(utils.UnIndent(*value))
		}

		return value
	}

	for i, extraArgs := range conf.Terraform.ExtraArgs {
		substitute(&extraArgs.Description)
		conf.Terraform.ExtraArgs[i] = extraArgs
	}

	substitute(conf.Uniqueness)
	substitute(conf.AssumeRole)
	if conf.Terraform != nil {
		substitute(&conf.Terraform.Source)
	}
	if conf.RemoteState != nil && conf.RemoteState.Config != nil {
		for key, value := range conf.RemoteState.Config {
			switch val := value.(type) {
			case string:
				conf.RemoteState.Config[key] = *substitute(&val)
			}
		}
	}

	substituteHooks := func(hooks HookList) {
		for i, hook := range hooks {
			substitute(&hook.Command)
			substitute(&hook.Description)
			for i, arg := range hook.Arguments {
				hook.Arguments[i] = *substitute(&arg)
			}
			hooks[i] = hook
		}
	}
	substituteHooks(conf.PreHooks)
	substituteHooks(conf.PostHooks)

	for i, command := range conf.ExtraCommands {
		substitute(&command.Description)
		for i, cmd := range command.Commands {
			command.Commands[i] = *substitute(&cmd)
		}
		for i, arg := range command.Arguments {
			command.Arguments[i] = *substitute(&arg)
		}
		conf.ExtraCommands[i] = command
	}

	for i, importer := range conf.ImportFiles {
		substitute(&importer.Description)
		substitute(&importer.Source)
		substitute(&importer.Target)
		for i, value := range importer.Files {
			importer.Files[i] = *substitute(&value)
		}
		for _, value := range importer.CopyAndRename {
			substitute(&value.Source)
			substitute(&value.Target)
		}
		conf.ImportFiles[i] = importer
	}
}
