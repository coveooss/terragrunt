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
	gotemplateHcl "github.com/coveooss/gotemplate/v3/hcl"
	"github.com/coveooss/gotemplate/v3/json"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/gotemplate/v3/yaml"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/remote"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/fatih/color"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

const (
	// DefaultConfigName is the name of the default file name where to store terragrunt definitions
	DefaultConfigName = options.DefaultConfigName

	// TerragruntScriptFolder is the name of the scripts folder generated under the temporary terragrunt folder
	TerragruntScriptFolder = ".terragrunt-scripts"
)

// TerragruntConfig represents a parsed and expanded configuration
type TerragruntConfig struct {
	ApprovalConfig  ApprovalConfigList  `hcl:"approval_config,block" export:"true"`
	Dependencies    *ModuleDependencies `hcl:"dependencies,block" export:"true"`
	ExportVariables ExportVariablesList `hcl:"export_variables,block" export:"true"`
	ExportConfig    ExportConfigList    `hcl:"export_config,block" export:"true"`
	ExtraArgs       ExtraArgumentsList  `hcl:"extra_arguments,block" export:"true"`
	ExtraCommands   ExtraCommandList    `hcl:"extra_command,block" export:"true"`
	ImportFiles     ImportFilesList     `hcl:"import_files,block" export:"true"`
	ImportVariables ImportVariablesList `hcl:"import_variables,block" export:"true"`
	RunConditions   RunConditionList    `hcl:"run_conditions,block"`
	PreHooks        PreHookList         `hcl:"pre_hook,block" export:"true"`
	PostHooks       PostHookList        `hcl:"post_hook,block" export:"true"`
	RemoteState     *remote.State       `hcl:"remote_state,block" export:"true"`
	Terraform       *TerraformConfig    `hcl:"terraform,block" export:"true"`

	AssumeRoleDurationHours *int      `hcl:"assume_role_duration_hours,attr" export:"true"`
	AssumeRoleHclDefinition cty.Value `hcl:"assume_role,optional"`
	Description             string    `hcl:"description,optional" export:"true"`
	InputsHclDefinition     cty.Value `hcl:"inputs,optional"`
	UniquenessCriteria      *string   `hcl:"uniqueness_criteria,attr" export:"true"`

	AssumeRole []string `export:"true"`
	Inputs     map[string]interface{}

	options *options.TerragruntOptions
}

func (conf TerragruntConfig) String() string {
	return collections.PrettyPrintStruct(conf)
}

// ExtraArguments processes the extra_arguments defined in the terraform section of the config file
func (conf TerragruntConfig) ExtraArguments(source string) ([]string, error) {
	return conf.ExtraArgs.Apply(source)
}

// AsDictionary exports tagged (export: true) properties of the configuration as a dictionary
func (conf TerragruntConfig) AsDictionary() (result collections.IDictionary, err error) {
	defer tgerrors.Recover(func(recovered error) { err = recovered })

	result = collections.CreateDictionary()

	v := reflect.ValueOf(conf)
	t := reflect.TypeOf(conf)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		if !field.CanInterface() || field.IsZero() || field.Interface() == nil {
			continue
		}
		if export, found := fieldType.Tag.Lookup("export"); export != "true" || !found {
			continue
		}
		result.Set(fieldType.Name, field.Interface())
	}
	return
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

// TerragruntConfigFile represents the configuration supported in a Terragrunt configuration file (i.e. terragrunt.hcl)
type TerragruntConfigFile struct {
	Path string
	// remain will send everything that isn't match into the labelled struct
	// In that case, most of the config goes down to TerragruntConfig
	// https://godoc.org/github.com/hashicorp/hcl/v2/gohcl
	TerragruntConfig `hcl:",squash"`
	Include          *IncludeConfig `hcl:"include,block"`
}

func (tcf *TerragruntConfigFile) init()         { tcf.TerragruntConfig.init(tcf) }
func (tcf TerragruntConfigFile) String() string { return collections.PrettyPrintStruct(tcf) }

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
	if tcf.Terraform != nil && len(tcf.Terraform.LegacyExtraArgs) > 0 {
		tcf.Terraform.LegacyExtraArgs.init(tcf)
		terragruntOptions.Logger.Warning(color.HiRedString("Specifying extra_arguments in the terraform block is deprecated, move the statement in the root document: %s", tcf.Terraform.LegacyExtraArgs))
		tcf.ExtraArgs = append(tcf.ExtraArgs, tcf.Terraform.LegacyExtraArgs...)
		tcf.Terraform.LegacyExtraArgs = nil
		if tcf.Terraform.Source == "" {
			tcf.Terraform = nil
		}
	}

	if !tcf.InputsHclDefinition.IsNull() {
		if err := util.FromCtyValue(tcf.InputsHclDefinition, &tcf.Inputs); err != nil {
			return nil, err
		}
	}

	tcf.init()

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
func (tcf *TerragruntConfigFile) GetSourceFolder(name string, source string, failIfNotFound bool, fileRegex string) (string, error) {
	terragruntOptions := tcf.options

	if source != "" {
		sourceFolder, err := util.GetSource(source, filepath.Dir(tcf.Path), terragruntOptions.Logger, fileRegex)
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
	LegacyExtraArgs ExtraArgumentsList `hcl:"extra_arguments,block"` // Kept here only for retro compatibility
	Source          string             `hcl:"source,optional"`
}

func (conf TerraformConfig) String() string {
	return collections.PrettyPrintStruct(conf)
}

// ReadTerragruntConfig reads the Terragrunt config file from its default location
func ReadTerragruntConfig(terragruntOptions *options.TerragruntOptions) (*TerragruntConfig, error) {
	include := IncludeConfig{Path: terragruntOptions.TerragruntConfigPath}
	_, conf, err := ParseConfigFile(terragruntOptions, include)
	if err == nil {
		return conf, nil
	}
	switch tgerrors.Unwrap(err).(type) {
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
		if _, hasStack := err.(*tgerrors.Error); err != nil && !hasStack {
			err = tgerrors.WithStackTrace(err)
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

	if terragruntOptions.Logger.GetDefaultConsoleHookLevel() > logrus.TraceLevel {
		terragruntOptions.Logger.Tracef("Read configuration file at %s\n%s", include.Path, configString)
	}
	if terragruntOptions.ApplyTemplate {
		collections.SetListHelper(gotemplateHcl.GenericListHelper)
		collections.SetDictionaryHelper(gotemplateHcl.DictionaryHelper)

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
			if terragruntOptions.Logger.GetDefaultConsoleHookLevel() > logrus.TraceLevel {
				terragruntOptions.Logger.Tracef("Result:\n%s", configString)
			}
		} else {
			terragruntOptions.Logger.Tracef("Configuration file at %s was not modified by gotemplate", include.Path)
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

// Parse the Terragrunt config contained in the given string.
func parseConfigString(configString string, terragruntOptions *options.TerragruntOptions, include IncludeConfig) (config *TerragruntConfig, err error) {
	// We also support before_hook and after_hook to be compatible with upstream terragrunt
	// TODO: actually convert structure to ensure that fields are also compatible (i.e. commands => on_commands, execute[] => string, run_on_error => IgnoreError)
	configString = strings.Replace(configString, "before_hook", "pre_hook", -1)
	configString = strings.Replace(configString, "after_hook", "post_hook", -1)

	includeContext := &resolveContext{
		include: include,
		options: terragruntOptions,
	}
	var terragruntConfigFile *TerragruntConfigFile
	if terragruntConfigFile, err = parseConfigStringAsTerragruntConfig(configString, includeContext); err != nil {
		return nil, fmt.Errorf("caught error while parsing the Terragrunt config: %w", err)
	}

	if config, err = terragruntConfigFile.convertToTerragruntConfig(terragruntOptions); err != nil {
		return nil, fmt.Errorf("caught error while initializing the Terragrunt config: %w", err)
	}

	terragruntOptions.ImportVariablesMap(config.Inputs, options.ConfigVarFile)
	terragruntOptions.Logger.Tracef("Loaded configuration\n%v", color.HiYellowString(fmt.Sprint(terragruntConfigFile)))

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
	if err := parseHcl(configString, resolveContext.include.Path, &terragruntConfig, resolveContext); err != nil {
		return nil, err
	}
	terragruntConfig.Path = resolveContext.include.Path
	return &terragruntConfig, nil
}

// parseHcl uses the HCL2 parser to parse the given string into the struct specified by out.
// If the supplied data is in another format, it converts it to json to use HCL2 ParseJSON instea
func parseHcl(content string, filename string, out interface{}, resolveContext *resolveContext) (err error) {
	// The HCL2 parser and especially cty conversions will panic in many types of errors, so we have to recover from
	// those panics here and convert them to normal errors
	defer tgerrors.Recover(func(cause error) {
		err = tgerrors.WithStackTrace(panicWhileParsingConfig{RecoveredValue: cause, ConfigFile: filename})
	})

	parser := hclparse.NewParser()
	var file *hcl.File
	var parseDiagnostics hcl.Diagnostics

	convert := func(convert func([]byte, interface{}) error) error {
		var data map[string]interface{}
		if err := convert([]byte(content), &data); err != nil {
			return err
		}
		content, err := json.Marshal(data)
		if err != nil {
			panic(err)
		}
		// It is important to add an extension to the filename since hcl parsers maintain a cache of already seen files
		file, parseDiagnostics = parser.ParseJSON(content, filename+"retry")
		return nil
	}

	switch filepath.Ext(filename) {
	case ".json":
		file, parseDiagnostics = parser.ParseJSON([]byte(content), filename)
	case ".hcl", ".tfvars":
		file, parseDiagnostics = parser.ParseHCL([]byte(content), filename)
	case ".yml", ".yaml":
		if err := convert(yaml.Unmarshal); err != nil {
			return err
		}
	default:
		if file, parseDiagnostics = parser.ParseHCL([]byte(content), filename); parseDiagnostics != nil && parseDiagnostics.HasErrors() {
			// If the conversion did not succeed, we simply consider the error on ParseHCL as the final diagnostic
			convert(func(data []byte, out interface{}) error { return collections.ConvertData(string(data), out) })
		}
	}

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

			if !strings.HasSuffix(bootstrapDir, "/") {
				bootstrapDir = bootstrapDir + "/"
			}

			sourcePath, err := util.GetSource(bootstrapDir, terragruntOptions.WorkingDir, terragruntOptions.Logger, "")
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
				terragruntOptions.Logger.Tracef("Caught error while trying to load bootstrap file, trying parsing it as a variables file: %v", caughtError)
				variables, err := util.LoadVariablesFromSource(bootConfigString, bootstrapFile, terragruntOptions.WorkingDir, false, nil)
				terragruntOptions.ImportVariablesMap(variables, options.ConfigVarFile)
				if err != nil {
					err = fmt.Errorf("got error while parsing bootstrap config: %v\n then caught error while parsing it as a variables file: %w", caughtError, err)
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

	conf.merge(&includedConfig)
}

func (conf *TerragruntConfig) init(configFile *TerragruntConfigFile) {
	for i, t := 0, reflect.ValueOf(conf).Elem(); i < t.NumField(); i++ {
		if field := t.Field(i); field.CanSet() {
			if list, ok := field.Addr().Interface().(extensionListCompatible); ok {
				list.init(configFile)
			}
		}
	}
}

func (conf *TerragruntConfig) merge(other *TerragruntConfig) {
	confValue := reflect.ValueOf(conf).Elem()
	otherValue := reflect.ValueOf(other).Elem()
	for i := 0; i < confValue.NumField(); i++ {
		if field := confValue.Field(i); field.CanSet() {
			if list, ok := field.Addr().Interface().(extensionListCompatible); ok {
				list.merge(otherValue.Field(i).Addr().Interface().(extensionListCompatible).toGeneric())
			}
		}
	}
}

// Parse the config of the given include, if one is specified
func parseIncludedConfig(includedConfig *IncludeConfig, terragruntOptions *options.TerragruntOptions) (configString string, config *TerragruntConfig, err error) {
	if includedConfig.Path == "" && includedConfig.Source == "" {
		return "", nil, tgerrors.WithStackTrace(includedConfigMissingPath(terragruntOptions.TerragruntConfigPath))
	}

	if !filepath.IsAbs(includedConfig.Path) && includedConfig.Source == "" {
		includedConfig.Path = util.JoinPath(filepath.Dir(includedConfig.isIncludedBy.Path), includedConfig.Path)
	}

	return ParseConfigFile(terragruntOptions, *includedConfig)
}

type includedConfigMissingPath string

func (err includedConfigMissingPath) Error() string {
	return fmt.Sprintf("the include configuration in %s must specify a 'path' and/or 'source' parameter", string(err))
}

type panicWhileParsingConfig struct {
	ConfigFile     string
	RecoveredValue interface{}
}

func (err panicWhileParsingConfig) Error() string {
	return fmt.Sprintf("recovering panic while parsing '%s'. Got error of type '%v': %v", err.ConfigFile, reflect.TypeOf(err.RecoveredValue), err.RecoveredValue)
}
