package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/remote"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl"
)

const DefaultTerragruntConfigPath = "terraform.tfvars"
const OldTerragruntConfigPath = ".terragrunt"
const TerragruntScriptFolder = ".terragrunt-scripts"

// TerragruntConfig represents a parsed and expanded configuration
type TerragruntConfig struct {
	Description   string
	Terraform     *TerraformConfig
	RemoteState   *remote.RemoteState
	Dependencies  *ModuleDependencies
	Uniqueness    *string
	PreHooks      []Hook
	PostHooks     []Hook
	ExtraCommands []ExtraCommand
	ImportFiles   []ImportConfig
	AssumeRole    *string
}

func (conf *TerragruntConfig) String() string {
	return fmt.Sprintf("TerragruntConfig{Terraform = %v, RemoteState = %v, Dependencies = %v}", conf.Terraform, conf.RemoteState, conf.Dependencies)
}

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
			*value = strings.TrimSpace(util.UnIndent(*value))
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

	substituteHooks := func(hooks []Hook) {
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
		for _, value := range importer.CopyAndRenameFiles {
			substitute(&value.Source)
			substitute(&value.Target)
		}
		conf.ImportFiles[i] = importer
	}
}

// terragruntConfigFile represents the configuration supported in a Terragrunt configuration file (i.e.
// terraform.tfvars or .terragrunt)
type terragruntConfigFile struct {
	Description   string              `hcl:"description,omitempty"`
	Terraform     *TerraformConfig    `hcl:"terraform,omitempty"`
	Include       *IncludeConfig      `hcl:"include,omitempty"`
	Lock          *LockConfig         `hcl:"lock,omitempty"`
	RemoteState   *remote.RemoteState `hcl:"remote_state,omitempty"`
	Dependencies  *ModuleDependencies `hcl:"dependencies,omitempty"`
	Uniqueness    *string             `hcl:"uniqueness_criteria"`
	PreHooks      []Hook              `hcl:"pre_hooks,omitempty"`
	PostHooks     []Hook              `hcl:"post_hooks,omitempty"`
	ExtraCommands []ExtraCommand      `hcl:"extra_command,omitempty"`
	ImportFiles   []ImportConfig      `hcl:"import_files,omitempty"`
	AssumeRole    *string             `hcl:"assume_role"`
}

// Older versions of Terraform did not support locking, so Terragrunt offered locking as a feature. As of version 0.9.0,
// Terraform supports locking natively, so this feature was removed from Terragrunt. However, we keep around the
// LockConfig so we can log a warning for Terragrunt users who are still trying to use it.
type LockConfig map[interface{}]interface{}

// tfvarsFileWithTerragruntConfig represents a .tfvars file that contains a terragrunt = { ... } block
type tfvarsFileWithTerragruntConfig struct {
	Terragrunt *terragruntConfigFile `hcl:"terragrunt,omitempty"`
}

// IncludeConfig represents the configuration settings for a parent Terragrunt configuration file that you can
// "include" in a child Terragrunt configuration file
type IncludeConfig struct {
	Source    string `hcl:"source"`
	Path      string `hcl:"path"`
	IncludeBy *IncludeConfig
}

func (include IncludeConfig) String() string {
	var includeBy string
	if include.IncludeBy != nil {
		includeBy = fmt.Sprintf(" included by %v", include.IncludeBy)
	}
	return fmt.Sprintf("IncludeConfig: %v%s", util.JoinPath(include.Source, include.Path), includeBy)
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
	ExtraArgs []TerraformExtraArguments `hcl:"extra_arguments"`
	Source    string                    `hcl:"source"`
}

func (conf *TerraformConfig) String() string {
	return fmt.Sprintf("TerraformConfig{Source = %v}", conf.Source)
}

// TerraformExtraArguments sets a list of arguments to pass to Terraform if command fits any in the `Commands` list
type TerraformExtraArguments struct {
	Name             string   `hcl:",key"`
	Description      string   `hcl:"description"`
	Arguments        []string `hcl:"arguments,omitempty"`
	Vars             []string `hcl:"vars,omitempty"`
	RequiredVarFiles []string `hcl:"required_var_files,omitempty"`
	OptionalVarFiles []string `hcl:"optional_var_files,omitempty"`
	Commands         []string `hcl:"commands,omitempty"`
}

func (conf *TerraformExtraArguments) String() string {
	return fmt.Sprintf("TerraformArguments{Name = %s, Arguments = %v, Commands = %v}", conf.Name, conf.Arguments, conf.Commands)
}

// Hook is a definition of user command that should be executed as part of the terragrunt process
type Hook struct {
	Name           string   `hcl:",key"`
	Description    string   `hcl:"description"`
	Command        string   `hcl:"command"`
	OnCommands     []string `hcl:"on_commands,omitempty"`
	OS             []string `hcl:"os,omitempty"`
	Arguments      []string `hcl:"arguments,omitempty"`
	ExpandArgs     bool     `hcl:"expand_args,omitempty"`
	IgnoreError    bool     `hcl:"ignore_error,omitempty"`
	AfterInitState bool     `hcl:"after_init_state,omitempty"`
	Order          int      `hcl:"order,omitempty"`
}

func (hook *Hook) String() string {
	return fmt.Sprintf("Hook %s: %s %s", hook.Name, hook.Command, strings.Join(hook.Arguments, " "))
}

// ExtraCommand is a definition of user extra command that should be executed in place of terraform
type ExtraCommand struct {
	Name        string   `hcl:",key"`
	Description string   `hcl:"description"`
	Command     string   `hcl:"command,omitempty"`
	Commands    []string `hcl:"commands,omitempty"`
	OS          []string `hcl:"os,omitempty"`
	Aliases     []string `hcl:"aliases,omitempty"`
	Arguments   []string `hcl:"arguments,omitempty"`
	ExpandArgs  *bool    `hcl:"expand_args,omitempty"`
	UseState    *bool    `hcl:"use_state,omitempty"`
	ActAs       string   `hcl:"act_as,omitempty"`
	VersionArg  string   `hcl:"version,omitempty"`
}

func (command *ExtraCommand) String() string {
	return fmt.Sprintf("Extra Command %s: %s", command.Name, command.Commands)
}

// ImportConfig is a configuration of files that must be imported from another directory to the terraform directory
// prior executing terraform commands
type ImportConfig struct {
	Name               string          `hcl:",key"`
	Description        string          `hcl:"description"`
	Source             string          `hcl:"source"`
	Files              []string        `hcl:"files"`
	CopyAndRenameFiles []CopyAndRename `hcl:"copy_and_rename"`
	Required           *bool           `hcl:"required,omitempty"`
	ImportIntoModules  bool            `hcl:"import_into_modules"`
	FileMode           *int            `hcl:"file_mode,omitempty"`
	Target             string          `hcl:"target,omitempty"`
	Prefix             *string         `hcl:"prefix,omitempty"`
	OS                 []string        `hcl:"os,omitempty"`
}

// CopyAndRename is a structure used by ImportConfig to rename the imported files
type CopyAndRename struct {
	Source string `hcl:"source"`
	Target string `hcl:"target"`
}

func (importConfig ImportConfig) String() string {
	files := importConfig.Files

	for _, copy := range importConfig.CopyAndRenameFiles {
		files = append(files, fmt.Sprintf("%s → %s", copy.Source, copy.Target))
	}

	return fmt.Sprintf("ImportConfig %s %s required=%v modules=%v : %s",
		importConfig.Name, importConfig.Source,
		importConfig.Required, importConfig.ImportIntoModules,
		strings.Join(files, ", "))
}

// Returns the default path to use for the Terragrunt configuration file. The reason this is a method rather than a
// constant is that older versions of Terragrunt stored configuration in a different file. This method returns the
// path to the old configuration format if such a file exists and the new format otherwise.
func DefaultConfigPath(workingDir string) string {
	path := util.JoinPath(workingDir, OldTerragruntConfigPath)
	if util.FileExists(path) {
		return path
	}
	return util.JoinPath(workingDir, DefaultTerragruntConfigPath)
}

// Returns a list of all Terragrunt config files in the given path or any subfolder of the path. A file is a Terragrunt
// config file if it has a name as returned by the DefaultConfigPath method and contains Terragrunt config contents
// as returned by the IsTerragruntConfigFile method.
func FindConfigFilesInPath(terragruntOptions *options.TerragruntOptions) ([]string, error) {
	rootPath := terragruntOptions.WorkingDir
	configFiles := []string{}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if util.FileExists(filepath.Join(path, "terragrunt.ignore")) {
				// If we wish to exclude a directory from the *-all commands, we just
				// have to put an empty file name terragrunt.ignore in the folder
				return nil
			}
			if terragruntOptions.NonInteractive && util.FileExists(filepath.Join(path, "terragrunt-non-interactive.ignore")) {
				// If we wish to exclude a directory from the *-all commands, we just
				// have to put an empty file name terragrunt-non-interactive.ignore in
				// the folder
				return nil
			}
			configPath := DefaultConfigPath(path)
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

// Returns true if the given path corresponds to file that could be a Terragrunt config file. A file could be a
// Terragrunt config file if:
//
// 1. The file exists
// 2. It is a .terragrunt file, which is the old Terragrunt-specific file format
// 3. The file contains HCL contents with a terragrunt = { ... } block
func IsTerragruntConfigFile(path string) (bool, error) {
	if !util.FileExists(path) {
		return false, nil
	}

	if isOldTerragruntConfig(path) {
		return true, nil
	}

	return isNewTerragruntConfig(path)
}

// Returns true if the given path points to an old Terragrunt config file
func isOldTerragruntConfig(path string) bool {
	return strings.HasSuffix(path, OldTerragruntConfigPath)
}

// Returns true if the given path points to a new (current) Terragrunt config file
func isNewTerragruntConfig(path string) (bool, error) {
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

// Read the Terragrunt config file from its default location
func ReadTerragruntConfig(terragruntOptions *options.TerragruntOptions) (*TerragruntConfig, error) {
	return ParseConfigFile(terragruntOptions, IncludeConfig{Path: terragruntOptions.TerragruntConfigPath})
}

// Parse the Terragrunt config file at the given path. If the include parameter is not nil, then treat this as a config
// included in some other config file when resolving relative paths.
func ParseConfigFile(terragruntOptions *options.TerragruntOptions, include IncludeConfig) (config *TerragruntConfig, err error) {
	if include.Path == "" {
		include.Path = DefaultTerragruntConfigPath
	}

	if include.IncludeBy == nil {
		// Check if the config has already been loaded
		if include.Source == "" {
			if include.Path, err = util.CanonicalPath(include.Path, ""); err != nil {
				return
			}
		}
		var exist bool
		config, exist = configFiles[include.Path]
		if exist {
			terragruntOptions.Logger.Debugf("Config already in the cache %s", include.Path)
			return
		}
	}

	if isOldTerragruntConfig(include.Path) {
		terragruntOptions.Logger.Warningf("DEPRECATION : Found deprecated config file format %s. This old config format will not be supported in the future. Please move your config files into a %s file.", include.Path, DefaultTerragruntConfigPath)
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

	terragruntOptions.Logger.Infof("Reading Terragrunt config file at %s", util.GetPathRelativeToWorkingDirMax(source, 2))

	terragruntOptions.ImportVariables(configString, source, options.ConfigVarFile)
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

	if include.IncludeBy == nil {
		configFiles[include.Path] = config
	}

	return
}

var configFiles = make(map[string]*TerragruntConfig)

// Parse the Terragrunt config contained in the given string.
func parseConfigString(configString string, terragruntOptions *options.TerragruntOptions, include IncludeConfig) (config *TerragruntConfig, err error) {
	resolvedConfigString, err := ResolveTerragruntConfigString(configString, include, terragruntOptions)
	if err != nil {
		return
	}

	terragruntConfigFile, err := parseConfigStringAsTerragruntConfigFile(resolvedConfigString, include.Path)
	if err != nil {
		return
	}
	if terragruntConfigFile == nil {
		err = errors.WithStackTrace(CouldNotResolveTerragruntConfigInFile(include.Path))
		return
	}

	config, err = terragruntConfigFile.convertToTerragruntConfig(terragruntOptions)
	if err != nil || terragruntConfigFile.Include == nil {
		return
	}

	terragruntConfigFile.Include.IncludeBy = &include

	includedConfig, err := parseIncludedConfig(terragruntConfigFile.Include, terragruntOptions)
	if err != nil {
		return
	}

	config.mergeIncludedConfig(*includedConfig, terragruntOptions)
	return
}

// Parse the given config string, read from the given config file, as a terragruntConfigFile struct. This method solely
// converts the HCL syntax in the string to the terragruntConfigFile struct; it does not process any interpolations.
func parseConfigStringAsTerragruntConfigFile(configString string, configPath string) (*terragruntConfigFile, error) {
	if isOldTerragruntConfig(configPath) {
		terragruntConfig := &terragruntConfigFile{}
		if err := hcl.Decode(terragruntConfig, configString); err != nil {
			return nil, errors.WithStackTrace(err)
		}
		return terragruntConfig, nil
	}

	tfvarsConfig := &tfvarsFileWithTerragruntConfig{}
	if err := hcl.Decode(tfvarsConfig, configString); err != nil {
		return nil, errors.WithStackTrace(err)
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

	logFunc := terragruntOptions.Logger.Debugf

	if includedConfig.Terraform != nil {
		if conf.Terraform == nil {
			conf.Terraform = includedConfig.Terraform
		} else {
			if conf.Terraform.Source == "" {
				conf.Terraform.Source = includedConfig.Terraform.Source
			}

			conf.mergeExtraArgs(includedConfig.Terraform.ExtraArgs, logFunc)
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

	conf.mergeImports(includedConfig.ImportFiles, logFunc)
	conf.mergeExtraCommands(includedConfig.ExtraCommands, logFunc)
	conf.mergeHooks(&conf.PreHooks, includedConfig.PreHooks, mergeModePrepend, logFunc)
	conf.mergeHooks(&conf.PostHooks, includedConfig.PostHooks, mergeModeAppend, logFunc)
}

type mergeMode int

const (
	mergeModePrepend mergeMode = iota
	mergeModeAppend
)

// Merge the extra arguments priorising those defined in the leaf
func (conf *TerragruntConfig) mergeHooks(hookSource *[]Hook, imported []Hook, mode mergeMode, log func(string, ...interface{})) {
	source := *hookSource
	defer func() { *hookSource = source }()
	var argType string
	if hookSource == &conf.PreHooks {
		argType = "pre_hook"
	} else {
		argType = "post_hook"
	}

	if len(imported) == 0 {
		return
	} else if len(source) == 0 {
		source = imported
		return
	}

	// Create a map with existing elements
	index := make(map[string]int, len(source))
	for i, hook := range source {
		index[hook.Name] = i
	}

	// Create a list of the hooks that should be added to the list
	new := make([]Hook, 0, len(imported))
	for _, element := range imported {
		if pos, exist := index[element.Name]; exist {
			// It already exist in the list, so is is an override, we remove it from its current position
			// and add it to the list of newly addd elements to keep its original declaration ordering.
			new = append(new, source[pos])
			delete(index, element.Name)
			log("Skipping %s %v as it is overridden in the current config", argType, element.Name)
		} else {
			new = append(new, element)
		}
	}

	if len(index) != len(source) {
		// Some elements must bre removed from the original list, we must
		newSource := make([]Hook, 0, len(index))
		for _, element := range source {
			if _, found := index[element.Name]; found {
				newSource = append(newSource, element)
			}
		}
		source = newSource
	}

	if mode == mergeModeAppend {
		source = append(source, new...)
	} else {
		source = append(new, source...)
	}
}

// Merge the extra arguments priorising those defined in the leaf
func (conf *TerragruntConfig) mergeExtraCommands(imported []ExtraCommand, log func(string, ...interface{})) {
	source := conf.ExtraCommands
	argType := "extra_command"
	defer func() { conf.ExtraCommands = source }()

	if len(imported) == 0 {
		return
	} else if len(source) == 0 {
		source = imported
		return
	}

	// Create a map with existing elements
	index := make(map[string]int, len(source))
	for i, hook := range source {
		index[hook.Name] = i
	}

	// Create a list of the hooks that should be added to the list
	new := make([]ExtraCommand, 0, len(imported))
	for _, element := range imported {
		if pos, exist := index[element.Name]; exist {
			// It already exist in the list, so is is an override, we remove it from its current position
			// and add it to the list of newly addd elements to keep its original declaration ordering.
			new = append(new, source[pos])
			delete(index, element.Name)
			log("Skipping %s %v as it is overridden in the current config", argType, element.Name)
		} else {
			new = append(new, element)
		}
	}

	if len(index) != len(source) {
		// Some elements must bre removed from the original list, we must
		newSource := make([]ExtraCommand, 0, len(index))
		for _, element := range source {
			if _, found := index[element.Name]; found {
				newSource = append(newSource, element)
			}
		}
		source = newSource
	}

	source = append(new, source...)
}

// Merge the import files priorising those defined in the leaf
func (conf *TerragruntConfig) mergeImports(imported []ImportConfig, log func(string, ...interface{})) {
	source := conf.ImportFiles
	argType := "import_files"
	defer func() { conf.ImportFiles = source }()

	if len(imported) == 0 {
		return
	} else if len(source) == 0 {
		source = imported
		return
	}

	// Create a map with existing elements
	index := make(map[string]int, len(source))
	for i, hook := range source {
		index[hook.Name] = i
	}

	// Create a list of the hooks that should be added to the list
	new := make([]ImportConfig, 0, len(imported))
	for _, element := range imported {
		if pos, exist := index[element.Name]; exist {
			// It already exist in the list, so is is an override, we remove it from its current position
			// and add it to the list of newly addd elements to keep its original declaration ordering.
			new = append(new, source[pos])
			delete(index, element.Name)
			log("Skipping %s %v as it is overridden in the current config", argType, element.Name)
		} else {
			new = append(new, element)
		}
	}

	if len(index) != len(source) {
		// Some elements must bre removed from the original list, we must
		newSource := make([]ImportConfig, 0, len(index))
		for _, element := range source {
			if _, found := index[element.Name]; found {
				newSource = append(newSource, element)
			}
		}
		source = newSource
	}

	source = append(new, source...)
}

// Merge the extra arguments priorising those defined in the leaf
func (conf *TerragruntConfig) mergeExtraArgs(imported []TerraformExtraArguments, log func(string, ...interface{})) {
	source := conf.Terraform.ExtraArgs
	argType := "import_files"
	defer func() { conf.Terraform.ExtraArgs = source }()

	if len(imported) == 0 {
		return
	} else if len(source) == 0 {
		source = imported
		return
	}

	// Create a map with existing elements
	index := make(map[string]int, len(source))
	for i, hook := range source {
		index[hook.Name] = i
	}

	// Create a list of the hooks that should be added to the list
	new := make([]TerraformExtraArguments, 0, len(imported))
	for _, element := range imported {
		if pos, exist := index[element.Name]; exist {
			// It already exist in the list, so is is an override, we remove it from its current position
			// and add it to the list of newly addd elements to keep its original declaration ordering.
			new = append(new, source[pos])
			delete(index, element.Name)
			log("Skipping %s %v as it is overridden in the current config", argType, element.Name)
		} else {
			new = append(new, element)
		}
	}

	if len(index) != len(source) {
		// Some elements must bre removed from the original list, we must
		newSource := make([]TerraformExtraArguments, 0, len(index))
		for _, element := range source {
			if _, found := index[element.Name]; found {
				newSource = append(newSource, element)
			}
		}
		source = newSource
	}

	source = append(new, source...)
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
		includedConfig.Path = util.JoinPath(filepath.Dir(includedConfig.IncludeBy.Path), includedConfig.Path)
	}

	return ParseConfigFile(terragruntOptions, *includedConfig)
}

// Convert the contents of a fully resolved Terragrunt configuration to a TerragruntConfig object
func (configFromFile terragruntConfigFile) convertToTerragruntConfig(terragruntOptions *options.TerragruntOptions) (*TerragruntConfig, error) {
	terragruntConfig := &TerragruntConfig{}

	if configFromFile.Lock != nil {
		terragruntOptions.Logger.Warningf("Found a lock configuration in the Terraform configuration at %s. Terraform added native support for locking as of version 0.9.0, so this feature has been removed from Terragrunt and will have no effect. See your Terraform backend docs for how to configure locking: https://www.terraform.io/docs/backends/types/index.html.", terragruntOptions.TerragruntConfigPath)
	}

	if configFromFile.RemoteState != nil {
		configFromFile.RemoteState.FillDefaults()
		if err := configFromFile.RemoteState.Validate(); err != nil {
			return nil, err
		}

		terragruntConfig.RemoteState = configFromFile.RemoteState
	}

	terragruntConfig.Description = configFromFile.Description
	terragruntConfig.Terraform = configFromFile.Terraform
	terragruntConfig.Dependencies = configFromFile.Dependencies
	terragruntConfig.Uniqueness = configFromFile.Uniqueness
	terragruntConfig.PreHooks = configFromFile.PreHooks
	terragruntConfig.PostHooks = configFromFile.PostHooks
	terragruntConfig.ExtraCommands = configFromFile.ExtraCommands
	terragruntConfig.ImportFiles = configFromFile.ImportFiles
	terragruntConfig.AssumeRole = configFromFile.AssumeRole

	return terragruntConfig, nil
}

// Custom error types

type IncludedConfigMissingPath string

func (err IncludedConfigMissingPath) Error() string {
	return fmt.Sprintf("The include configuration in %s must specify a 'path' and/or 'source' parameter", string(err))
}

type CouldNotResolveTerragruntConfigInFile string

func (err CouldNotResolveTerragruntConfigInFile) Error() string {
	return fmt.Sprintf("Could not find Terragrunt configuration settings in %s", string(err))
}
