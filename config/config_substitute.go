package config

import (
	"path/filepath"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
)

// SubstituteAllVariables replace all remaining variables by the value
func (conf *TerragruntConfig) SubstituteAllVariables() {
	conf.substitute(conf.Uniqueness)

	if roles, ok := conf.AssumeRole.([]string); ok {
		for i := range roles {
			conf.substitute(&roles[i])
		}
		conf.AssumeRole = roles
	}

	if conf.RemoteState != nil && conf.RemoteState.Config != nil {
		for key, value := range conf.RemoteState.Config {
			switch val := value.(type) {
			case string:
				conf.RemoteState.Config[key] = *conf.substitute(&val)
			}
		}
	}

	if conf.Terraform != nil {
		conf.substitute(&conf.Terraform.Source)
	}
	for i := range conf.ExtraArgs {
		conf.ExtraArgs[i].substituteVars()
	}
	for i := range conf.PreHooks {
		conf.PreHooks[i].substituteVars()
	}
	for i := range conf.PostHooks {
		conf.PostHooks[i].substituteVars()
	}
	for i := range conf.ExtraCommands {
		conf.ExtraCommands[i].substituteVars()
	}
	for i := range conf.ImportFiles {
		conf.ImportFiles[i].substituteVars()
	}
	for i := range conf.ImportVariables {
		conf.ImportVariables[i].substituteVars()
	}
}

// substitute is an helper function to convert string in a configuration structure
func (conf *TerragruntConfig) substitute(value *string) *string {
	if value == nil {
		return nil
	}

	*value = SubstituteVars(*value, conf.options)
	if !conf.options.IgnoreRemainingInterpolation {
		// We only substitute folders on the last substitute call
		*value = strings.Replace(*value, getTempFolder, conf.options.DownloadDir, -1)
		*value = strings.Replace(*value, getScriptsFolder, filepath.Join(conf.options.WorkingDir, TerragruntScriptFolder), -1)
		*value = strings.TrimSpace(collections.UnIndent(*value))
	}

	return value
}

// substituteEnv is an helper function to convert a map of key/value strings in a configuration structure
func (conf *TerragruntConfig) substituteEnv(env map[string]string) {
	for k, v := range env {
		env[k] = *conf.substitute(&v)
	}
}
