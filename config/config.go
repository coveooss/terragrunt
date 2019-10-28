package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/remote"
	"github.com/gruntwork-io/terragrunt/util"
)

const (
	// DefaultTerragruntConfigPath is the name of the default file name where to store terragrunt definitions
	DefaultTerragruntConfigPath = "terraform.tfvars"

	// TerragruntScriptFolder is the name of the scripts folder generated under the temporary terragrunt folder
	TerragruntScriptFolder = ".terragrunt-scripts"
)

// TerragruntConfig represents a parsed and expanded configuration
type TerragruntConfig struct {
	Description     string                      `hcl:"description"`
	RunConditions   RunConditions               `hcl:"run_conditions"`
	Terraform       *TerraformConfig            `hcl:"terraform"`
	RemoteState     *remote.RemoteState         `hcl:"remote_state"`
	Dependencies    *ModuleDependencies         `hcl:"dependencies"`
	Uniqueness      *string                     `hcl:"uniqueness_criteria"`
	AssumeRole      interface{}                 `hcl:"assume_role"`
	ExtraArgs       TerraformExtraArgumentsList `hcl:"extra_arguments"`
	PreHooks        HookList                    `hcl:"pre_hook"`
	PostHooks       HookList                    `hcl:"post_hook"`
	ExtraCommands   ExtraCommandList            `hcl:"extra_command"`
	ImportFiles     ImportFilesList             `hcl:"import_files"`
	ApprovalConfig  ApprovalConfigList          `hcl:"approval_config"`
	ImportVariables ImportVariablesList         `hcl:"import_variables"`

	options *options.TerragruntOptions
}

func (conf TerragruntConfig) String() string {
	return collections.PrettyPrintStruct(conf)
}

// ExtraArguments processes the extra_arguments defined in the terraform section of the config file
func (conf TerragruntConfig) ExtraArguments(source string) ([]string, error) {
	return conf.ExtraArgs.Filter(source)
}

func (conf TerragruntConfig) globFiles(pattern string, stopOnMatch bool, folders ...string) (result []string) {
	conf.substitute(&pattern)
	if filepath.IsAbs(pattern) {
		return utils.GlobFuncTrim(pattern)
	}
	for i := range folders {
		result = append(result, utils.GlobFuncTrim(filepath.Join(folders[i], pattern))...)
		if stopOnMatch && len(result) > 0 {
			// If the pattern matches files and stopOnMatch is true, we stop looking for other folders
			break
		}
	}
	return
}

// TerragruntConfigFile represents the configuration supported in a Terragrunt configuration file (i.e. terraform.tfvars or .terragrunt)
type TerragruntConfigFile struct {
	TerragruntConfig `hcl:",squash"`
	Include          *IncludeConfig
	Lock             *LockConfig
	Path             string
}

func (tcf TerragruntConfigFile) String() string {
	return collections.PrettyPrintStruct(tcf)
}

// Convert the contents of a fully resolved Terragrunt configuration to a TerragruntConfig object
func (tcf *TerragruntConfigFile) convertToTerragruntConfig(terragruntOptions *options.TerragruntOptions) (config *TerragruntConfig, err error) {
	if tcf.Lock != nil {
		terragruntOptions.Logger.Warningf(""+
			"Found a lock configuration in the Terraform configuration at %s. Terraform added native support for locking as "+
			"of version 0.9.0, so this feature has been removed from Terragrunt and will have no effect. See your Terraform "+
			"backend docs for how to configure locking: https://www.terraform.io/docs/backends/types/index.html.",
			terragruntOptions.TerragruntConfigPath)
	}

	if tcf.RemoteState != nil {
		tcf.RemoteState.FillDefaults()
		if err = tcf.RemoteState.Validate(); err != nil {
			return
		}
	}

	switch role := tcf.AssumeRole.(type) {
	case nil:
		break
	case string:
		// A single role is specified, we convert it in an array of roles
		tcf.AssumeRole = []string{role}
	case []interface{}:
		// We convert the array to an array of string
		roles := make([]string, len(role))
		for i := range role {
			roles[i] = fmt.Sprint(role[i])
		}
		tcf.AssumeRole = roles
	default:
		terragruntOptions.Logger.Errorf("Invalid configuration for assume_role, must be either a string or a list of strings: %[1]v (%[1]T)", role)
	}

	// Make the context available to sub-objects
	tcf.options = terragruntOptions

	// We combine extra arguments defined in terraform block into the extra arguments defined in the terragrunt block
	if tcf.Terraform != nil {
		tcf.ExtraArgs = append(tcf.ExtraArgs, tcf.Terraform.LegacyExtraArgs...)
	}

	tcf.ExtraArgs.init(tcf)
	tcf.ExtraCommands.init(tcf)
	tcf.ImportFiles.init(tcf)
	tcf.ImportVariables.init(tcf)
	tcf.ApprovalConfig.init(tcf)
	tcf.PreHooks.init(tcf)
	tcf.PostHooks.init(tcf)
	err = tcf.RunConditions.init(tcf.options)

	if tcf.Include == nil {
		// If the newly loaded configuration file is not to be merged, we force the merge
		// process to ensure that duplicated elements will be properly processed
		newConfig := &TerragruntConfig{options: tcf.options}
		newConfig.mergeIncludedConfig(tcf.TerragruntConfig, terragruntOptions)
		return newConfig, err
	}
	return &tcf.TerragruntConfig, err
}

// GetSourceFolder resolves remote source and returns the local temporary folder
func (tcf *TerragruntConfigFile) GetSourceFolder(name string, source string, failIfNotFound bool) (string, error) {
	terragruntOptions := tcf.options

	if source != "" {
		tcf.substitute(&source)
		sourceFolder, err := util.GetSource(source, filepath.Dir(tcf.Path), terragruntOptions.Logger)
		if err != nil {
			if failIfNotFound {
				return "", err
			}
			terragruntOptions.Logger.Warningf("%s: %s could not be fetched: %v", name, source, err)
		}
		return sourceFolder, nil
	}

	return "", nil
}

// LockConfig is older versions of Terraform did not support locking, so Terragrunt offered locking as a feature. As of version 0.9.0,
// Terraform supports locking natively, so this feature was removed from Terragrunt. However, we keep around the
// LockConfig so we can log a warning for Terragrunt users who are still trying to use it.
type LockConfig map[interface{}]interface{}

// tfvarsFileWithTerragruntConfig represents a .tfvars file that contains a terragrunt = { ... } block
type tfvarsFileWithTerragruntConfig struct {
	Terragrunt *TerragruntConfigFile `hcl:"terragrunt,omitempty"`
}

// IncludeConfig represents the configuration settings for a parent Terragrunt configuration file that you can
// "include" in a child Terragrunt configuration file
type IncludeConfig struct {
	Source       string `hcl:"source"`
	Path         string `hcl:"path"`
	isIncludedBy *IncludeConfig
	isBootstrap  bool
}

func (include IncludeConfig) String() string {
	var includeBy string
	if include.isIncludedBy != nil {
		includeBy = fmt.Sprintf(" included by %v", include.isIncludedBy)
	}
	return fmt.Sprintf("%v%s", util.JoinPath(include.Source, include.Path), includeBy)
}

// ModuleDependencies represents the paths to other Terraform modules that must be applied before the current module
// can be applied
type ModuleDependencies struct {
	Paths []string `hcl:"paths"`
}

func (deps *ModuleDependencies) String() string {
	return fmt.Sprintf("ModuleDependencies{Paths = %v}", deps.Paths)
}

// TerraformConfig specifies where to find the Terraform configuration files
type TerraformConfig struct {
	LegacyExtraArgs TerraformExtraArgumentsList `hcl:"extra_arguments"` // Kept here only for retro compatibility
	Source          string                      `hcl:"source"`
}

func (conf TerraformConfig) String() string {
	return collections.PrettyPrintStruct(conf)
}

// FindConfigFilesInPath returns a list of all Terragrunt config files in the given path or any subfolder of the path.
// A file is a Terragrunt config file if it its name matches the DefaultTerragruntConfigPath constant and contains Terragrunt
// config contents as returned by the IsTerragruntConfigFile method.
func FindConfigFilesInPath(terragruntOptions *options.TerragruntOptions) ([]string, error) {
	rootPath := terragruntOptions.WorkingDir
	configFiles := []string{}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if util.FileExists(filepath.Join(path, options.IgnoreFile)) {
				// If we wish to exclude a directory from the *-all commands, we just
				// have to put an empty file name terragrunt.ignore in the folder
				return nil
			}
			if terragruntOptions.NonInteractive && util.FileExists(filepath.Join(path, options.IgnoreFileNonInteractive)) {
				// If we wish to exclude a directory from the *-all commands, we just
				// have to put an empty file name terragrunt-non-interactive.ignore in
				// the folder
				return nil
			}
			configPath := util.JoinPath(path, DefaultTerragruntConfigPath)
			isTerragruntConfig, err := IsTerragruntConfigFile(configPath)
			if err != nil {
				return err
			}
			if isTerragruntConfig {
				configFiles = append(configFiles, configPath)
			}
		}

		return nil
	})

	return configFiles, err
}

// IsTerragruntConfigFile returns true if the given path corresponds to file that could be a Terragrunt config file.
// A file could be a Terragrunt config file if:
//   1. The file exists
//   3. The file contains HCL contents with a terragrunt = { ... } block
func IsTerragruntConfigFile(path string) (bool, error) {
	if !util.FileExists(path) {
		return false, nil
	}

	configContents, err := util.ReadFileAsString(path)
	if err != nil {
		return false, err
	}

	return containsTerragruntBlock(configContents), nil
}

// Returns true if the given string contains valid HCL with a terragrunt = { ... } block
func containsTerragruntBlock(configString string) bool {
	terragruntConfig := &tfvarsFileWithTerragruntConfig{}
	if err := hcl.Decode(terragruntConfig, configString); err != nil {
		return false
	}
	return terragruntConfig.Terragrunt != nil
}

// ReadTerragruntConfig reads the Terragrunt config file from its default location
func ReadTerragruntConfig(terragruntOptions *options.TerragruntOptions) (*TerragruntConfig, error) {
	include := IncludeConfig{Path: terragruntOptions.TerragruntConfigPath}
	conf, err := ParseConfigFile(terragruntOptions, include)
	if err == nil {
		return conf, nil
	}
	switch errors.Unwrap(err).(type) {
	case CouldNotResolveTerragruntConfigInFile:
		terragruntOptions.Logger.Warningf("No terragrunt section in %s, assuming default values", terragruntOptions.TerragruntConfigPath)
	case *os.PathError:
		stat, _ := os.Stat(filepath.Dir(terragruntOptions.TerragruntConfigPath))
		if stat == nil || !stat.IsDir() {
			return nil, err
		}
		terragruntOptions.Logger.Warningf("File %s not found, assuming default values", terragruntOptions.TerragruntConfigPath)
	default:
		return nil, err
	}
	// The configuration has not been initialized, we generate a default configuration
	return parseConfigString("terragrunt{}", terragruntOptions, include)
}

// ParseConfigFile parses the Terragrunt config file at the given path. If the include parameter is not nil, then treat
// this as a config included in some other config file when resolving relative paths.
func ParseConfigFile(terragruntOptions *options.TerragruntOptions, include IncludeConfig) (config *TerragruntConfig, err error) {
	defer func() {
		if _, hasStack := err.(*errors.Error); err != nil && !hasStack {
			err = errors.WithStackTrace(err)
		}
	}()

	if include.Path == "" {
		include.Path = DefaultTerragruntConfigPath
	}

	if include.isIncludedBy == nil {
		// Check if the config has already been loaded
		if include.Source == "" {
			if include.Path, err = util.CanonicalPath(include.Path, ""); err != nil {
				return
			}
		}
		if cached, _ := configFiles.Load(include.Path); cached != nil {
			terragruntOptions.Logger.Debugf("Config already in the cache %s", include.Path)
			return cached.(*TerragruntConfig), nil
		}
	}

	var configString, source string
	if include.Source == "" {
		configString, err = util.ReadFileAsString(include.Path)
		source = include.Path
	} else {
		include.Path, configString, err = util.ReadFileAsStringFromSource(include.Source, include.Path, terragruntOptions.Logger)
		source = include.Path
	}
	if err != nil {
		return nil, err
	}

	if util.ApplyTemplate() {
		collections.ListHelper = hcl.GenericListHelper
		collections.DictionaryHelper = hcl.DictionaryHelper

		var t *template.Template
		if t, err = template.NewTemplate(terragruntOptions.WorkingDir, terragruntOptions.GetContext(), "", nil); err != nil {
			return
		}

		// Add interpolation functions directly to gotemplate
		// We must create a new context to ensure that the functions are added to the right template since they are
		// folder dependant
		includeContext := &resolveContext{
			include: include,
			options: terragruntOptions,
		}
		t.GetNewContext(filepath.Dir(source), true).AddFunctions(includeContext.getHelperFunctions(), "Terragrunt", nil)

		if configString, err = t.ProcessContent(configString, source); err != nil {
			return
		}
	}

	var terragrunt interface{}
	if terragrunt, err = terragruntOptions.ImportVariables(configString, source, options.ConfigVarFile); err != nil {
		return
	}

	terragruntOptions.TerragruntRawConfig, _ = collections.TryAsDictionary(terragrunt)

	if config, err = parseConfigString(configString, terragruntOptions, include); err != nil {
		return
	}

	if config.Dependencies != nil {
		// We should convert all dependencies to absolute path
		folder := filepath.Dir(source)
		for i, dep := range config.Dependencies.Paths {
			if !filepath.IsAbs(dep) {
				dep, err = filepath.Abs(filepath.Join(folder, dep))
				config.Dependencies.Paths[i] = dep
			}
		}
	}

	if include.isIncludedBy == nil {
		configFiles.Store(include.Path, config)
	}

	return
}

var configFiles sync.Map
var hookWarningGiven, lockWarningGiven bool

// Parse the Terragrunt config contained in the given string.
func parseConfigString(configString string, terragruntOptions *options.TerragruntOptions, include IncludeConfig) (config *TerragruntConfig, err error) {
	configString, err = ResolveTerragruntConfigString(configString, include, terragruntOptions)
	if err != nil {
		return
	}

	// We also support before_hook and after_hook to be compatible with upstream terragrunt
	// TODO: actually convert structure to ensure that fields are also compatible (i.e. commands => on_commands, execute[] => string, run_on_error => IgnoreError)
	configString = strings.Replace(configString, "before_hook", "pre_hook", -1)
	configString = strings.Replace(configString, "after_hook", "post_hook", -1)

	before := configString
	// pre_hooks & post_hooks have been renamed to pre_hook & post_hook, we support old naming to avoid breaking change
	configString = strings.Replace(configString, "pre_hooks", "pre_hook", -1)
	configString = strings.Replace(configString, "post_hooks", "post_hook", -1)
	if !hookWarningGiven && before != configString {
		// We should issue this warning only once
		hookWarningGiven = true
		terragruntOptions.Logger.Warning("pre_hooks/post_hooks are deprecated, please use pre_hook/post_hook instead")
	}

	before = configString
	configString = strings.Replace(configString, "lock_table", "dynamodb_table", -1)
	if !lockWarningGiven && before != configString {
		// We should issue this warning only once
		lockWarningGiven = true
		terragruntOptions.Logger.Warning("lock_table is deprecated, please use dynamodb_table instead")
	}

	terragruntConfigFile, err := parseConfigStringAsTerragruntConfigFile(configString, include.Path)
	if err != nil {
		return
	}
	if terragruntConfigFile == nil {
		err = errors.WithStackTrace(CouldNotResolveTerragruntConfigInFile(include.Path))
		return
	}

	if config, err = terragruntConfigFile.convertToTerragruntConfig(terragruntOptions); err != nil {
		return
	}
	terragruntOptions.Logger.Infof("Loaded configuration\n%v", color.GreenString(fmt.Sprint(terragruntConfigFile)))

	if !path.IsAbs(include.Path) {
		include.Path, _ = filepath.Abs(include.Path)
	}

	if terragruntConfigFile.Include == nil {
		if include.isBootstrap {
			// This is already a bootstrap file, so we stop the inclusion here
			return
		}
		terragruntConfigFile.Include = &(IncludeConfig{
			isBootstrap:  true,
			isIncludedBy: &include,
		})
		// We check if we should merge bootstrap files defined by environment variable TERRAGRUNT_BOOT_CONFIGS
		paths := strings.Split(os.Getenv(options.EnvBootConfigs), string(os.PathListSeparator))
		for _, bootstrapFile := range collections.AsList(paths).Reverse().Strings() {
			bootstrapFile = strings.TrimSpace(bootstrapFile)
			if bootstrapFile != "" {
				stat, _ := os.Stat(bootstrapFile)
				if stat == nil || stat.IsDir() {
					terragruntConfigFile.Include.Source = bootstrapFile
				} else {
					terragruntConfigFile.Include.Source = path.Dir(bootstrapFile)
					terragruntConfigFile.Include.Path = path.Base(bootstrapFile)
				}
				var bootConfig *TerragruntConfig
				if bootConfig, err = parseIncludedConfig(terragruntConfigFile.Include, terragruntOptions); err != nil {
					return
				}
				config.mergeIncludedConfig(*bootConfig, terragruntOptions)
			}
		}
		return
	}

	terragruntConfigFile.Include.isIncludedBy = &include
	includedConfig, err := parseIncludedConfig(terragruntConfigFile.Include, terragruntOptions)
	if err != nil {
		return
	}

	config.mergeIncludedConfig(*includedConfig, terragruntOptions)
	return
}

// Parse the given config string, read from the given config file, as a terragruntConfigFile struct. This method solely
// converts the HCL syntax in the string to the terragruntConfigFile struct; it does not process any interpolations.
func parseConfigStringAsTerragruntConfigFile(configString string, configPath string) (*TerragruntConfigFile, error) {
	tfvarsConfig := &tfvarsFileWithTerragruntConfig{}
	if err := hcl.Decode(tfvarsConfig, configString); err != nil {
		return nil, errors.WithStackTrace(err)
	}
	if tfvarsConfig.Terragrunt != nil {
		tfvarsConfig.Terragrunt.Path = configPath
	}
	return tfvarsConfig.Terragrunt, nil
}

// Merge an included config into the current config. Some elements specified in both config will be merged while
// others will be overridded only if they are not already specified in the original config.
func (conf *TerragruntConfig) mergeIncludedConfig(includedConfig TerragruntConfig, terragruntOptions *options.TerragruntOptions) {
	if includedConfig.Description != "" {
		if conf.Description != "" {
			conf.Description += "\n"
		}
		conf.Description += includedConfig.Description
	}

	if conf.RemoteState == nil {
		conf.RemoteState = includedConfig.RemoteState
	}

	if includedConfig.Terraform != nil {
		if conf.Terraform == nil {
			conf.Terraform = includedConfig.Terraform
		} else {
			if conf.Terraform.Source == "" {
				conf.Terraform.Source = includedConfig.Terraform.Source
			}
		}
	}

	if conf.Dependencies == nil {
		conf.Dependencies = includedConfig.Dependencies
	} else if includedConfig.Dependencies != nil {
		conf.Dependencies.Paths = append(conf.Dependencies.Paths, includedConfig.Dependencies.Paths...)
	}

	if conf.Uniqueness == nil {
		conf.Uniqueness = includedConfig.Uniqueness
	}

	if conf.AssumeRole == nil {
		conf.AssumeRole = includedConfig.AssumeRole
	}

	conf.ExtraArgs.Merge(includedConfig.ExtraArgs)
	conf.RunConditions.Merge(includedConfig.RunConditions)
	conf.ImportFiles.Merge(includedConfig.ImportFiles)
	conf.ImportVariables.Merge(includedConfig.ImportVariables)
	conf.ExtraCommands.Merge(includedConfig.ExtraCommands)
	conf.ApprovalConfig.Merge(includedConfig.ApprovalConfig)
	conf.PreHooks.MergePrepend(includedConfig.PreHooks)
	conf.PostHooks.MergeAppend(includedConfig.PostHooks)
}

// Parse the config of the given include, if one is specified
func parseIncludedConfig(includedConfig *IncludeConfig, terragruntOptions *options.TerragruntOptions) (config *TerragruntConfig, err error) {
	if includedConfig.Path == "" && includedConfig.Source == "" {
		return nil, errors.WithStackTrace(IncludedConfigMissingPath(terragruntOptions.TerragruntConfigPath))
	}

	includedConfig.Path, err = ResolveTerragruntConfigString(includedConfig.Path, *includedConfig, terragruntOptions)
	if err != nil {
		return nil, err
	}
	includedConfig.Source, err = ResolveTerragruntConfigString(includedConfig.Source, *includedConfig, terragruntOptions)
	if err != nil {
		return nil, err
	}

	if !filepath.IsAbs(includedConfig.Path) && includedConfig.Source == "" {
		includedConfig.Path = util.JoinPath(filepath.Dir(includedConfig.isIncludedBy.Path), includedConfig.Path)
	}

	return ParseConfigFile(terragruntOptions, *includedConfig)
}

// IncludedConfigMissingPath is the error returned when there is no path defined in the include directive
type IncludedConfigMissingPath string

func (err IncludedConfigMissingPath) Error() string {
	return fmt.Sprintf("The include configuration in %s must specify a 'path' and/or 'source' parameter", string(err))
}

// CouldNotResolveTerragruntConfigInFile is the error returned when the configuration file could not be resolved
type CouldNotResolveTerragruntConfigInFile string

func (err CouldNotResolveTerragruntConfigInFile) Error() string {
	return fmt.Sprintf("Could not find Terragrunt configuration settings in %s", string(err))
}
