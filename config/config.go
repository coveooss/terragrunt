package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
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
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

const (
	// DefaultTerragruntConfigPath is the name of the default file name where to store terragrunt definitions
	DefaultTerragruntConfigPath = "terragrunt.hcl"

	// TerragruntScriptFolder is the name of the scripts folder generated under the temporary terragrunt folder
	TerragruntScriptFolder = ".terragrunt-scripts"
)

// TerragruntConfig represents a parsed and expanded configuration
type TerragruntConfig struct {
	ApprovalConfig          ApprovalConfigList `hcl:"approval_config,block"`
	AssumeRole              []string
	AssumeRoleDurationHours *int                        `hcl:"assume_role_duration_hours,attr"`
	Dependencies            *ModuleDependencies         `hcl:"dependencies,block"`
	Description             string                      `hcl:"description,optional"`
	ExportVariablesConfigs  []ExportVariablesConfig     `hcl:"export_variables,block"`
	ExtraArgs               TerraformExtraArgumentsList `hcl:"extra_arguments,block"`
	ExtraCommands           ExtraCommandList            `hcl:"extra_command,block"`
	ImportFiles             ImportFilesList             `hcl:"import_files,block"`
	ImportVariables         ImportVariablesList         `hcl:"import_variables,block"`
	Inputs                  map[string]interface{}
	PreHooks                HookList      `hcl:"pre_hook,block"`
	PostHooks               HookList      `hcl:"post_hook,block"`
	RemoteState             *remote.State `hcl:"remote_state,block"`
	RunConditions           RunConditions
	Terraform               *TerraformConfig `hcl:"terraform,block"`
	UniquenessCriteria      *string          `hcl:"uniqueness_criteria,attr"`

	AssumeRoleHclDefinition    cty.Value                    `hcl:"assume_role,optional"`
	InputsHclDefinition        cty.Value                    `hcl:"inputs,optional"`
	RunConditionsHclDefinition []runConditionsHclDefinition `hcl:"run_conditions,block"`

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

// TerragruntConfigFile represents the configuration supported in a Terragrunt configuration file (i.e. terragrunt.hcl or .terragrunt)
type TerragruntConfigFile struct {
	Path string
	// remain will send everything that isn't match into the labelled struct
	// In that case, most of the config goes down to TerragruntConfig
	// https://godoc.org/github.com/hashicorp/hcl2/gohcl
	TerragruntConfig `hcl:",remain"`
	Include          *IncludeConfig `hcl:"include,block"`
}

func (tcf TerragruntConfigFile) String() string {
	return collections.PrettyPrintStruct(tcf)
}

// Convert the contents of a fully resolved Terragrunt configuration to a TerragruntConfig object
func (tcf *TerragruntConfigFile) convertToTerragruntConfig(terragruntOptions *options.TerragruntOptions) (config *TerragruntConfig, err error) {
	if tcf.RemoteState != nil {
		if err = tcf.RemoteState.Validate(); err != nil {
			return
		}
	}

	if !tcf.AssumeRoleHclDefinition.IsNull() {
		if tcf.AssumeRoleHclDefinition.Type() == cty.String {
			tcf.AssumeRole = []string{tcf.AssumeRoleHclDefinition.AsString()}
		} else if tcf.AssumeRoleHclDefinition.Type().IsTupleType() || tcf.AssumeRoleHclDefinition.Type().IsListType() {
			tcf.AssumeRole, _ = ctySliceToStringSlice(tcf.AssumeRoleHclDefinition.AsValueSlice())
		} else {
			terragruntOptions.Logger.Errorf("Invalid configuration for assume_role, must be either a string or a list of strings: %[1]v (%[1]v)", tcf.AssumeRoleHclDefinition, tcf.AssumeRoleHclDefinition.Type())
		}
	}

	// Make the context available to sub-objects
	tcf.options = terragruntOptions

	// We combine extra arguments defined in terraform block into the extra arguments defined in the terragrunt block
	if tcf.Terraform != nil {
		tcf.ExtraArgs = append(tcf.ExtraArgs, tcf.Terraform.LegacyExtraArgs...)
	}

	if !tcf.InputsHclDefinition.IsNull() {
		if err := util.FromCtyValue(tcf.InputsHclDefinition, &tcf.Inputs); err != nil {
			return nil, err
		}
	}

	tcf.ExtraArgs.init(tcf)
	tcf.ExtraCommands.init(tcf)
	tcf.ImportFiles.init(tcf)
	tcf.ImportVariables.init(tcf)
	tcf.ApprovalConfig.init(tcf)
	tcf.PreHooks.init(tcf)
	tcf.PostHooks.init(tcf)
	tcf.RunConditions = RunConditions{}
	for _, condition := range tcf.RunConditionsHclDefinition {
		if !condition.RunIf.IsNull() {
			var runIfCondition map[string]interface{}
			if err := util.FromCtyValue(condition.RunIf, &runIfCondition); err != nil {
				return nil, err
			}
			tcf.RunConditions.Allows = append(tcf.RunConditions.Allows, runIfCondition)
		}

		if !condition.IgnoreIf.IsNull() {
			var ignoreIfCondition map[string]interface{}
			if err := util.FromCtyValue(condition.IgnoreIf, &ignoreIfCondition); err != nil {
				return nil, err
			}
			tcf.RunConditions.Denies = append(tcf.RunConditions.Denies, ignoreIfCondition)
		}
	}
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
// If the source is local, it is directly returned
func (tcf *TerragruntConfigFile) GetSourceFolder(name string, source string, failIfNotFound bool) (string, error) {
	terragruntOptions := tcf.options

	if source != "" {
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

// IncludeConfig represents the configuration settings for a parent Terragrunt configuration file that you can
// "include" in a child Terragrunt configuration file
type IncludeConfig struct {
	Source       string `hcl:"source,optional"`
	Path         string `hcl:"path,optional"`
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
	LegacyExtraArgs TerraformExtraArgumentsList `hcl:"extra_arguments,block"` // Kept here only for retro compatibility
	Source          string                      `hcl:"source,optional"`
}

func (conf TerraformConfig) String() string {
	return collections.PrettyPrintStruct(conf)
}

// FindConfigFilesInPath returns a list of all Terragrunt config files in the given path or any subfolder of the path.
// A file is a Terragrunt config file if it its name matches the DefaultTerragruntConfigPath constant and contains Terragrunt
// config contents as returned by the IsTerragruntConfig method.
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
			configPath := terragruntOptions.ConfigPath(path)
			if _, err := os.Stat(configPath); err == nil {
				configFiles = append(configFiles, configPath)
			}
		}

		return nil
	})

	return configFiles, err
}

// ReadTerragruntConfig reads the Terragrunt config file from its default location
func ReadTerragruntConfig(terragruntOptions *options.TerragruntOptions) (*TerragruntConfig, error) {
	include := IncludeConfig{Path: terragruntOptions.TerragruntConfigPath}
	_, conf, err := ParseConfigFile(terragruntOptions, include)
	if err == nil {
		return conf, nil
	}
	switch errors.Unwrap(err).(type) {
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
	return parseConfigString("", terragruntOptions, include)
}

// ParseConfigFile parses the Terragrunt config file at the given path. If the include parameter is not nil, then treat
// this as a config included in some other config file when resolving relative paths.
func ParseConfigFile(terragruntOptions *options.TerragruntOptions, include IncludeConfig) (configString string, config *TerragruntConfig, err error) {
	defer func() {
		if _, hasStack := err.(*errors.Error); err != nil && !hasStack {
			err = errors.WithStackTrace(err)
		}
	}()

	if include.Path == "" {
		include.Path = filepath.Base(terragruntOptions.TerragruntConfigPath)
	}

	if include.isIncludedBy == nil && !include.isBootstrap {
		// Check if the config has already been loaded
		if include.Source == "" {
			if include.Path, err = util.CanonicalPath(include.Path, ""); err != nil {
				return
			}
		}
		if cached, _ := configFiles.Load(include.Path); cached != nil {
			cachedList := cached.([]interface{})
			terragruntOptions.Logger.Debugf("Config already in the cache %s", include.Path)
			return cachedList[0].(string), cachedList[1].(*TerragruntConfig), nil
		}
	}

	config = &TerragruntConfig{options: terragruntOptions}
	if include.isIncludedBy == nil && !include.isBootstrap {
		if err = config.loadBootConfigs(terragruntOptions, &IncludeConfig{isBootstrap: true}, terragruntOptions.PreBootConfigurationPaths); err != nil {
			terragruntOptions.Logger.Debugf("Error parsing preboot configuration files: %v", err)
			return
		}
	}

	var source string
	if include.Source == "" {
		configString, err = util.ReadFileAsString(include.Path)
		source = include.Path
	} else {
		include.Path, configString, err = util.ReadFileAsStringFromSource(include.Source, include.Path, terragruntOptions.Logger)
		source = include.Path
	}
	if err != nil {
		return "", nil, err
	}

	terragruntOptions.Logger.Tracef("Read configuration file at %s\n%s", include.Path, configString)
	if terragruntOptions.ApplyTemplate {
		collections.SetListHelper(hcl.GenericListHelper)
		collections.SetDictionaryHelper(hcl.DictionaryHelper)

		var t *template.Template
		options := template.DefaultOptions()
		if t, err = template.NewTemplate(terragruntOptions.WorkingDir, terragruntOptions.GetContext(), "", options); err != nil {
			terragruntOptions.Logger.Debugf("Error creating template for %s: %v", terragruntOptions.WorkingDir, err)
			return
		}

		// Add interpolation functions directly to gotemplate
		// We must create a new context to ensure that the functions are added to the right template since they are
		// folder dependant
		includeContext := &resolveContext{
			include: include,
			options: terragruntOptions,
		}
		t.GetNewContext(filepath.Dir(source), true).AddFunctions(includeContext.getHelperFunctionsInterfaces(), "Terragrunt", nil)

		oldConfigString := configString
		if configString, err = t.ProcessContent(configString, source); err != nil {
			terragruntOptions.Logger.Debugf("Error running gotemplate on %s: %v", include.Path, err)
			return
		}

		if oldConfigString != configString {
			terragruntOptions.Logger.Debugf("Configuration file at %s was modified by gotemplate", include.Path)
			terragruntOptions.Logger.Tracef("Result:\n%s", configString)
		} else {
			terragruntOptions.Logger.Debugf("Configuration file at %s was not modified by gotemplate", include.Path)

		}
	}

	var userConfig *TerragruntConfig
	if userConfig, err = parseConfigString(configString, terragruntOptions, include); err != nil || userConfig == nil {
		return
	}

	if userConfig.Dependencies != nil {
		// We should convert all dependencies to absolute path
		folder := filepath.Dir(source)
		for i, dep := range userConfig.Dependencies.Paths {
			if !filepath.IsAbs(dep) {
				dep, err = filepath.Abs(filepath.Join(folder, dep))
				userConfig.Dependencies.Paths[i] = dep
			}
		}
	}
	config.mergeIncludedConfig(*userConfig, terragruntOptions)

	if include.isIncludedBy == nil {
		configFiles.Store(include.Path, []interface{}{configString, config})
	}

	return
}

var configFiles sync.Map
var hookWarningGiven bool

// Parse the Terragrunt config contained in the given string.
func parseConfigString(configString string, terragruntOptions *options.TerragruntOptions, include IncludeConfig) (config *TerragruntConfig, err error) {
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

	includeContext := &resolveContext{
		include: include,
		options: terragruntOptions,
	}
	var terragruntConfigFile *TerragruntConfigFile
	if terragruntConfigFile, err = parseConfigStringAsTerragruntConfig(configString, includeContext); err != nil {
		return nil, fmt.Errorf("caught error while parsing the Terragrunt config: %s", err)
	}

	if config, err = terragruntConfigFile.convertToTerragruntConfig(terragruntOptions); err != nil {
		return nil, fmt.Errorf("caught error while initializing the Terragrunt config: %s", err)
	}

	terragruntOptions.ImportVariablesMap(config.Inputs, options.ConfigVarFile)
	terragruntOptions.Logger.Debugf("Loaded configuration\n%v", color.GreenString(fmt.Sprint(terragruntConfigFile)))

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

		err = config.loadBootConfigs(terragruntOptions, terragruntConfigFile.Include, terragruntOptions.BootConfigurationPaths)
		return
	}

	terragruntConfigFile.Include.isIncludedBy = &include
	_, includedConfig, err := parseIncludedConfig(terragruntConfigFile.Include, terragruntOptions)
	if err != nil {
		return
	}

	config.mergeIncludedConfig(*includedConfig, terragruntOptions)
	return
}

// Parse the given config string, read from the given config file, as a terragruntConfigFile struct. This method solely
// converts the HCL syntax in the string to the terragruntConfigFile struct; it does not process any interpolations.
func parseConfigStringAsTerragruntConfig(configString string, resolveContext *resolveContext) (*TerragruntConfigFile, error) {
	terragruntConfig := TerragruntConfigFile{}
	err := parseHcl(configString, resolveContext.include.Path, &terragruntConfig, resolveContext)
	if err != nil {
		return nil, err
	}
	terragruntConfig.Path = resolveContext.include.Path
	return &terragruntConfig, nil
}

// parseHcl uses the HCL2 parser to parse the given string into the struct specified by out.
func parseHcl(hcl string, filename string, out interface{}, resolveContext *resolveContext) (err error) {
	// The HCL2 parser and especially cty conversions will panic in many types of errors, so we have to recover from
	// those panics here and convert them to normal errors
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.WithStackTrace(PanicWhileParsingConfig{RecoveredValue: recovered, ConfigFile: filename})
		}
	}()

	parser := hclparse.NewParser()

	file, parseDiagnostics := parser.ParseHCL([]byte(hcl), filename)
	if parseDiagnostics != nil && parseDiagnostics.HasErrors() {
		return parseDiagnostics
	}

	funcs, err := resolveContext.getHelperFunctionsHCLContext()
	if err != nil {
		return err
	}
	if decodeDiagnostics := gohcl.DecodeBody(file.Body, funcs, out); decodeDiagnostics != nil && decodeDiagnostics.HasErrors() {
		return decodeDiagnostics
	}

	return nil
}

func (conf *TerragruntConfig) loadBootConfigs(terragruntOptions *options.TerragruntOptions, include *IncludeConfig, bootConfigsPath []string) error {
	for _, bootstrapFile := range collections.AsList(bootConfigsPath).Reverse().Strings() {
		bootstrapFile = strings.TrimSpace(bootstrapFile)
		if bootstrapFile != "" {
			bootstrapDir := path.Dir(bootstrapFile)
			if strings.HasPrefix(bootstrapDir, "s3:/") {
				// The path.Dir removes the double slash, so s3:// is not longer interpretated correctly
				bootstrapDir = strings.Replace(bootstrapDir, "s3:/", "s3://", -1)
			}

			sourcePath, err := util.GetSource(bootstrapDir, terragruntOptions.WorkingDir, terragruntOptions.Logger)
			if err != nil {
				return err
			}
			include.Source = sourcePath
			include.Path = path.Base(bootstrapFile)
			bootstrapFile = path.Join(sourcePath, include.Path)
			var (
				bootConfig       *TerragruntConfig
				bootConfigString string
			)
			if bootConfigString, bootConfig, err = parseIncludedConfig(include, terragruntOptions); err != nil {
				if path.Ext(bootstrapFile) == ".hcl" {
					// This is a config file, config parsing has to succeed
					return err
				}
				caughtError := err
				terragruntOptions.Logger.Debugf("Caught error while trying to load bootstrap file, trying parsing it as a variables file: %v", caughtError)
				variables, err := util.LoadVariablesFromSource(bootConfigString, bootstrapFile, terragruntOptions.WorkingDir, false, nil)
				terragruntOptions.ImportVariablesMap(variables, options.ConfigVarFile)
				if err != nil {
					err = fmt.Errorf("got error while parsing bootstrap config: %v\n then caught error while parsing it as a variables file: %v", caughtError, err)
					return err
				}
				continue
			}
			conf.mergeIncludedConfig(*bootConfig, terragruntOptions)
		}
	}
	return nil
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

	if conf.UniquenessCriteria == nil {
		conf.UniquenessCriteria = includedConfig.UniquenessCriteria
	}

	if conf.AssumeRole == nil {
		conf.AssumeRole = includedConfig.AssumeRole
	}

	if conf.AssumeRoleDurationHours == nil {
		conf.AssumeRoleDurationHours = includedConfig.AssumeRoleDurationHours
	}

	if includedConfig.Inputs != nil {
		conf.Inputs, _ = utils.MergeDictionaries(conf.Inputs, includedConfig.Inputs)

	}

	if conf.ExportVariablesConfigs == nil {
		conf.ExportVariablesConfigs = includedConfig.ExportVariablesConfigs
	} else if includedConfig.ExportVariablesConfigs != nil {
		conf.ExportVariablesConfigs = append(conf.ExportVariablesConfigs, includedConfig.ExportVariablesConfigs...)
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
func parseIncludedConfig(includedConfig *IncludeConfig, terragruntOptions *options.TerragruntOptions) (configString string, config *TerragruntConfig, err error) {
	if includedConfig.Path == "" && includedConfig.Source == "" {
		return "", nil, errors.WithStackTrace(IncludedConfigMissingPath(terragruntOptions.TerragruntConfigPath))
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

type ErrorParsingTerragruntConfig struct {
	ConfigPath string
	Underlying error
}

func (err ErrorParsingTerragruntConfig) Error() string {
	return fmt.Sprintf("Error parsing Terragrunt config at %s: %v", err.ConfigPath, err.Underlying)
}

type PanicWhileParsingConfig struct {
	ConfigFile     string
	RecoveredValue interface{}
}

func (err PanicWhileParsingConfig) Error() string {
	return fmt.Sprintf("Recovering panic while parsing '%s'. Got error of type '%v': %v", err.ConfigFile, reflect.TypeOf(err.RecoveredValue), err.RecoveredValue)
}

type InvalidBackendConfigType struct {
	ExpectedType string
	ActualType   string
}

func (err InvalidBackendConfigType) Error() string {
	return fmt.Sprintf("Expected backend config to be of type '%s' but got '%s'.", err.ExpectedType, err.ActualType)
}
