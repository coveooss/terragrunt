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

// TerragruntConfig represents a parsed and expanded configuration
type TerragruntConfig struct {
	Terraform    *TerraformConfig
	RemoteState  *remote.RemoteState
	Dependencies *ModuleDependencies
	Uniqueness   *string
	PreHooks     []Hook
	PostHooks    []Hook
	ImportFiles  []ImportConfig
	AssumeRole   *string
}

func (conf *TerragruntConfig) String() string {
	return fmt.Sprintf("TerragruntConfig{Terraform = %v, RemoteState = %v, Dependencies = %v}", conf.Terraform, conf.RemoteState, conf.Dependencies)
}

// SubstituteAllVariables replace all remaining variables by the value
func (conf *TerragruntConfig) SubstituteAllVariables(terragruntOptions *options.TerragruntOptions) {
	substitute := func(value *string) *string {
		if value == nil {
			return nil
		}

		*value = SubstituteVars(*value, terragruntOptions)
		return value
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
			for i, arg := range hook.Arguments {
				hook.Arguments[i] = *substitute(&arg)
			}
			hooks[i] = hook
		}
	}
	substituteHooks(conf.PreHooks)
	substituteHooks(conf.PostHooks)

	for i, importer := range conf.ImportFiles {
		substitute(&importer.Source)
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
	Terraform    *TerraformConfig    `hcl:"terraform,omitempty"`
	Include      *IncludeConfig      `hcl:"include,omitempty"`
	Lock         *LockConfig         `hcl:"lock,omitempty"`
	RemoteState  *remote.RemoteState `hcl:"remote_state,omitempty"`
	Dependencies *ModuleDependencies `hcl:"dependencies,omitempty"`
	Uniqueness   *string             `hcl:"uniqueness_criteria"`
	PreHooks     []Hook              `hcl:"pre_hooks,omitempty"`
	PostHooks    []Hook              `hcl:"post_hooks,omitempty"`
	ImportFiles  []ImportConfig      `hcl:"import_files,omitempty"`
	AssumeRole   *string             `hcl:"assume_role"`
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
	Name       string   `hcl:",key"`
	Command    string   `hcl:"command"`
	OnCommands []string `hcl:"on_commands,omitempty"`
	OS         []string `hcl:"os,omitempty"`
	Arguments  []string `hcl:"arguments,omitempty"`
	ExpandArgs bool     `hcl:"expand_args,omitempty"`
}

func (hook Hook) String() string {
	return fmt.Sprintf("Hook %s: %s %s", hook.Name, hook.Command, strings.Join(hook.Arguments, " "))
}

// ImportConfig is a configuration of files that must be imported from another directory to the terraform directory
// prior executing terraform commands
type ImportConfig struct {
	Name               string          `hcl:",key"`
	Source             string          `hcl:"source"`
	Files              []string        `hcl:"files"`
	CopyAndRenameFiles []CopyAndRename `hcl:"copy_and_rename"`
	Required           bool            `hcl:"required,omitempty"`
	ImportIntoModules  bool            `hcl:"import_into_modules"`
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
	terragruntOptions.Logger.Infof("Reading Terragrunt config file at %s", terragruntOptions.TerragruntConfigPath)
	return ParseConfigFile(terragruntOptions, IncludeConfig{Path: terragruntOptions.TerragruntConfigPath})
}

// Parse the Terragrunt config file at the given path. If the include parameter is not nil, then treat this as a config
// included in some other config file when resolving relative paths.
func ParseConfigFile(terragruntOptions *options.TerragruntOptions, include IncludeConfig) (*TerragruntConfig, error) {
	if isOldTerragruntConfig(include.Path) {
		terragruntOptions.Logger.Warningf("DEPRECATION : Found deprecated config file format %s. This old config format will not be supported in the future. Please move your config files into a %s file.", include.Path, DefaultTerragruntConfigPath)
	}

	var (
		configString string
		err          error
	)
	if include.Source == "" {
		configString, err = util.ReadFileAsString(include.Path)
	} else {
		if include.Path == "" {
			include.Path = DefaultTerragruntConfigPath
		}
		include.Path, configString, err = util.ReadFileAsStringFromSource(include.Source, include.Path, terragruntOptions.TerraformPath)
	}
	if err != nil {
		return nil, err
	}

	config, err := parseConfigString(configString, terragruntOptions, include)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// Parse the Terragrunt config contained in the given string.
func parseConfigString(configString string, terragruntOptions *options.TerragruntOptions, include IncludeConfig) (*TerragruntConfig, error) {
	resolvedConfigString, err := ResolveTerragruntConfigString(configString, include, terragruntOptions)
	if err != nil {
		return nil, err
	}

	terragruntConfigFile, err := parseConfigStringAsTerragruntConfigFile(resolvedConfigString, include.Path)
	if err != nil {
		return nil, err
	}
	if terragruntConfigFile == nil {
		return nil, errors.WithStackTrace(CouldNotResolveTerragruntConfigInFile(include.Path))
	}

	config, err := convertToTerragruntConfig(terragruntConfigFile, terragruntOptions)
	if err != nil {
		return nil, err
	}

	if terragruntConfigFile.Include == nil {
		return config, nil
	}

	terragruntConfigFile.Include.IncludeBy = &include

	includedConfig, err := parseIncludedConfig(terragruntConfigFile.Include, terragruntOptions)
	if err != nil {
		return nil, err
	}

	return mergeConfigWithIncludedConfig(config, includedConfig, terragruntOptions)
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
	} else {
		tfvarsConfig := &tfvarsFileWithTerragruntConfig{}
		if err := hcl.Decode(tfvarsConfig, configString); err != nil {
			return nil, errors.WithStackTrace(err)
		}
		return tfvarsConfig.Terragrunt, nil
	}
}

// Merge the given config with an included config. Anything specified in the current config will override the contents
// of the included config. If the included config is nil, just return the current config.
func mergeConfigWithIncludedConfig(config *TerragruntConfig, includedConfig *TerragruntConfig, terragruntOptions *options.TerragruntOptions) (*TerragruntConfig, error) {
	if includedConfig == nil {
		return config, nil
	}

	if config.RemoteState != nil {
		includedConfig.RemoteState = config.RemoteState
	}

	if config.Terraform != nil {
		if includedConfig.Terraform == nil {
			includedConfig.Terraform = config.Terraform
		} else {
			if config.Terraform.Source != "" {
				includedConfig.Terraform.Source = config.Terraform.Source
			}
			mergeExtraArgs(terragruntOptions, config.Terraform.ExtraArgs, &includedConfig.Terraform.ExtraArgs)
		}
	}

	if config.Dependencies != nil {
		includedConfig.Dependencies = config.Dependencies
	}

	if config.Uniqueness != nil {
		includedConfig.Uniqueness = config.Uniqueness
	}

	if config.AssumeRole != nil {
		includedConfig.AssumeRole = config.AssumeRole
	}

	mergePreHooks(terragruntOptions, config.PreHooks, &includedConfig.PreHooks)
	mergePostHooks(terragruntOptions, config.PostHooks, &includedConfig.PostHooks)
	mergeImports(terragruntOptions, config.ImportFiles, &includedConfig.ImportFiles)

	return includedConfig, nil
}

// Merge the extra arguments priorising those defined in the leaf
// func mergeExtraArgs(terragruntOptions *options.TerragruntOptions, original []TerraformExtraArguments, newExtra *[]TerraformExtraArguments) {
// 	result := *newExtra
// addExtra:
// 	for _, extra := range original {
// 		for i, existing := range result {
// 			if existing.Name == extra.Name {
// 				terragruntOptions.Logger.Infof("Skipping extra_arguments %v as it is overridden in the current config", extra.Name)
// 				// For extra args, we want to keep the values specified in the child and put them after
// 				// the parent ones, so if we encounter a duplicate, we just overwrite it.
// 				result[i] = extra
// 				continue addExtra
// 			}
// 		}
// 		result = append(result, extra)
// 	}
// 	*newExtra = result
// }

// Merge the extra arguments priorising those defined in the leaf
func mergePreHooks(terragruntOptions *options.TerragruntOptions, original []Hook, newHooks *[]Hook) {
	result := *newHooks
addHook:
	for _, hook := range original {
		for i, existing := range result {
			if existing.Name == hook.Name {
				terragruntOptions.Logger.Infof("Skipping Hook %v as it is overridden in the current config", hook.Name)
				result[i] = hook
				continue addHook
			}
		}
		result = append(result, hook)
	}
	*newHooks = result
}

func mergePostHooks(terragruntOptions *options.TerragruntOptions, original []Hook, newHooks *[]Hook) {
	result := original
addHook:
	for _, hook := range *newHooks {
		for _, existing := range original {
			if existing.Name == hook.Name {
				terragruntOptions.Logger.Infof("Skipping Hook %v as it is overridden in the current config", hook.Name)
				continue addHook
			}
		}
		result = append(result, hook)
	}
	*newHooks = result
}

// Merge the import files priorising those defined in the leaf
func mergeImports(terragruntOptions *options.TerragruntOptions, original []ImportConfig, newImports *[]ImportConfig) {
	result := *newImports
addImport:
	for _, importer := range original {
		for i, existing := range result {
			if existing.Name == importer.Name {
				terragruntOptions.Logger.Infof("Skipping ImportFiles %v as it is overridden in the current config", importer.Name)
				result[i] = importer
				continue addImport
			}
		}
		result = append(result, importer)
	}
	*newImports = result
}

// Merge the extra arguments.
//
// If a child's extra_arguments has the same name a parent's extra_arguments,
// then the child's extra_arguments will be selected (and the parent's ignored)
// If a child's extra_arguments has a different name from all of the parent's extra_arguments,
// then the child's extra_arguments will be added to the end  of the parents.
// Therefore, terragrunt will put the child extra_arguments after the parent's
// extra_arguments on the terraform cli.
// Therefore, if .tfvar files from both the parent and child contain a variable
// with the same name, the value from the child will win.
func mergeExtraArgs(terragruntOptions *options.TerragruntOptions, childExtraArgs []TerraformExtraArguments, parentExtraArgs *[]TerraformExtraArguments) {
	result := *parentExtraArgs
	for _, child := range childExtraArgs {
		parentExtraArgsWithSameName := getIndexOfExtraArgsWithName(result, child.Name)
		if parentExtraArgsWithSameName != -1 {
			// If the parent contains an extra_arguments with the same name as the child,
			// then override the parent's extra_arguments with the child's.
			terragruntOptions.Logger.Infof("extra_arguments '%v' from child overriding parent", child.Name)
			result[parentExtraArgsWithSameName] = child
		} else {
			// If the parent does not contain an extra_arguments with the same name as the child
			// then add the child to the end.
			// This ensures the child extra_arguments are added to the command line after the parent extra_arguments.
			result = append(result, child)
		}
	}
	*parentExtraArgs = result
}

// Returns the index of the extraArgs with the given name,
// or -1 if no extraArgs have the given name.
func getIndexOfExtraArgsWithName(extraArgs []TerraformExtraArguments, name string) int {
	for i, extra := range extraArgs {
		if extra.Name == name {
			return i
		}
	}
	return -1
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
func convertToTerragruntConfig(terragruntConfigFromFile *terragruntConfigFile, terragruntOptions *options.TerragruntOptions) (*TerragruntConfig, error) {
	terragruntConfig := &TerragruntConfig{}

	if terragruntConfigFromFile.Lock != nil {
		terragruntOptions.Logger.Warningf("Found a lock configuration in the Terraform configuration at %s. Terraform added native support for locking as of version 0.9.0, so this feature has been removed from Terragrunt and will have no effect. See your Terraform backend docs for how to configure locking: https://www.terraform.io/docs/backends/types/index.html.", terragruntOptions.TerragruntConfigPath)
	}

	if terragruntConfigFromFile.RemoteState != nil {
		terragruntConfigFromFile.RemoteState.FillDefaults()
		if err := terragruntConfigFromFile.RemoteState.Validate(); err != nil {
			return nil, err
		}

		terragruntConfig.RemoteState = terragruntConfigFromFile.RemoteState
	}

	terragruntConfig.Terraform = terragruntConfigFromFile.Terraform
	terragruntConfig.Dependencies = terragruntConfigFromFile.Dependencies
	terragruntConfig.Uniqueness = terragruntConfigFromFile.Uniqueness
	terragruntConfig.PreHooks = terragruntConfigFromFile.PreHooks
	terragruntConfig.PostHooks = terragruntConfigFromFile.PostHooks
	terragruntConfig.ImportFiles = terragruntConfigFromFile.ImportFiles
	terragruntConfig.AssumeRole = terragruntConfigFromFile.AssumeRole

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
