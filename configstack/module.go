package configstack

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
)

// Represents a single module (i.e. folder with Terraform templates), including the Terragrunt configuration for that
// module and the list of other modules that this module depends on
type TerraformModule struct {
	Path                 string
	Dependencies         []*TerraformModule
	Config               config.TerragruntConfig
	TerragruntOptions    *options.TerragruntOptions
	AssumeAlreadyApplied bool
}

// Render this module as a human-readable string
func (module TerraformModule) String() string {
	return fmt.Sprintf("Module %s (dependencies: [%s])", util.GetPathRelativeToWorkingDirMax(module.Path, 3), strings.Join(module.dependencies(), ", "))
}

// Run a module once all of its dependencies have finished executing.
func (module TerraformModule) dependencies() []string {
	result := make([]string, 0, len(module.Dependencies))
	for _, dep := range module.Dependencies {
		result = append(result, util.GetPathRelativeToWorkingDirMax(dep.Path, 3))
	}
	return result
}

// Simple returns a simplified version of the module with paths relative to working dir
func (module *TerraformModule) Simple() SimpleTerraformModule {
	dependencies := []string{}
	for _, dependency := range module.Dependencies {
		dependencies = append(dependencies, dependency.Path)
	}
	return SimpleTerraformModule{module.Path, dependencies}
}

// SimpleTerraformModule represents a simplified version of TerraformModule
type SimpleTerraformModule struct {
	Path         string   `json:"path"`
	Dependencies []string `json:"dependencies,omitempty" yaml:",omitempty"`
}

// SimpleTerraformModules represents a list of simplified version of TerraformModule
type SimpleTerraformModules []SimpleTerraformModule

// MakeRelative transforms each absolute path in relative path
func (modules SimpleTerraformModules) MakeRelative() (result SimpleTerraformModules) {
	result = make(SimpleTerraformModules, len(modules))
	for i := range modules {
		result[i].Path = util.GetPathRelativeToWorkingDir(modules[i].Path)
		result[i].Dependencies = make([]string, len(modules[i].Dependencies))
		for j := range result[i].Dependencies {
			result[i].Dependencies[j] = util.GetPathRelativeToWorkingDir(modules[i].Dependencies[j])
		}
	}
	return
}

// Go through each of the given Terragrunt configuration files and resolve the module that configuration file represents
// into a TerraformModule struct. Return the list of these TerraformModule structs.
func ResolveTerraformModules(terragruntConfigPaths []string, terragruntOptions *options.TerragruntOptions) ([]*TerraformModule, error) {
	canonicalTerragruntConfigPaths, err := util.CanonicalPaths(terragruntConfigPaths, ".")
	if err != nil {
		return []*TerraformModule{}, err
	}

	modules, err := resolveModules(canonicalTerragruntConfigPaths, terragruntOptions)
	if err != nil {
		return []*TerraformModule{}, err
	}

	externalDependencies, err := resolveExternalDependenciesForModules(canonicalTerragruntConfigPaths, modules, terragruntOptions)
	if err != nil {
		return []*TerraformModule{}, err
	}
	return crosslinkDependencies(mergeMaps(modules, externalDependencies), canonicalTerragruntConfigPaths)
}

// Go through each of the given Terragrunt configuration files and resolve the module that configuration file represents
// into a TerraformModule struct. Note that this method will NOT fill in the Dependencies field of the TerraformModule
// struct (see the crosslinkDependencies method for that). Return a map from module path to TerraformModule struct.
func resolveModules(canonicalTerragruntConfigPaths []string, terragruntOptions *options.TerragruntOptions) (map[string]*TerraformModule, error) {
	moduleMap := map[string]*TerraformModule{}

	for _, terragruntConfigPath := range canonicalTerragruntConfigPaths {
		module, err := resolveTerraformModule(terragruntConfigPath, terragruntOptions)
		if err != nil {
			return moduleMap, err
		}
		if module != nil {
			moduleMap[module.Path] = module
		}
	}

	return moduleMap, nil
}

// Create a TerraformModule struct for the Terraform module specified by the given Terragrunt configuration file path.
// Note that this method will NOT fill in the Dependencies field of the TerraformModule struct (see the
// crosslinkDependencies method for that).
func resolveTerraformModule(terragruntConfigPath string, terragruntOptions *options.TerragruntOptions) (*TerraformModule, error) {
	modulePath, err := util.CanonicalPath(filepath.Dir(terragruntConfigPath), ".")
	if err != nil {
		return nil, err
	}

	opts := terragruntOptions.Clone(terragruntConfigPath)
	terragruntConfig, err := config.ParseConfigFile(opts, config.IncludeConfig{Path: terragruntConfigPath})
	if err != nil {
		return nil, err
	}

	// Fix for https://github.com/gruntwork-io/terragrunt/issues/208
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(terragruntConfigPath), "*.tf"))
	if err != nil {
		return nil, err
	}
	if (terragruntConfig.Terraform == nil || terragruntConfig.Terraform.Source == "") && matches == nil {
		terragruntOptions.Logger.Debugf("Module %s does not have an associated terraform configuration and will be skipped.", filepath.Dir(terragruntConfigPath))
		return nil, nil
	}

	return &TerraformModule{Path: modulePath, Config: *terragruntConfig, TerragruntOptions: opts}, nil
}

// Look through the dependencies of the modules in the given map and resolve the "external" dependency paths listed in
// each modules config (i.e. those dependencies not in the given list of Terragrunt config canonical file paths).
// These external dependencies are outside of the current working directory, which means they may not be part of the
// environment the user is trying to apply-all or destroy-all. Therefore, this method also confirms whether the user wants
// to actually apply those dependencies or just assume they are already applied. Note that this method will NOT fill in
// the Dependencies field of the TerraformModule struct (see the crosslinkDependencies method for that).
func resolveExternalDependenciesForModules(canonicalTerragruntConfigPaths []string, moduleMap map[string]*TerraformModule, terragruntOptions *options.TerragruntOptions) (map[string]*TerraformModule, error) {
	allExternalDependencies := map[string]*TerraformModule{}

	for _, module := range moduleMap {
		externalDependencies, err := resolveExternalDependenciesForModule(module, canonicalTerragruntConfigPaths, terragruntOptions)
		if err != nil {
			return externalDependencies, err
		}

		for _, externalDependency := range externalDependencies {
			if _, alreadyFound := moduleMap[externalDependency.Path]; alreadyFound {
				continue
			}

			var expandDependencies []*TerraformModule
			for key, existingModule := range moduleMap {
				if strings.HasPrefix(key, externalDependency.Path) {
					expandDependencies = append(expandDependencies, existingModule)
				}
			}
			if len(expandDependencies) > 0 {
				module.Dependencies = append(module.Dependencies, expandDependencies...)

				subFoldersDependencies := make([]string, len(expandDependencies))
				for i := range expandDependencies {
					subFoldersDependencies[i] = expandDependencies[i].Path
				}
				module.Config.Dependencies.Paths = util.RemoveElementFromList(module.Config.Dependencies.Paths, externalDependency.Path)
				module.Config.Dependencies.Paths = append(module.Config.Dependencies.Paths, subFoldersDependencies...)
				continue
			}

			alreadyApplied, err := confirmExternalDependencyAlreadyApplied(module, externalDependency, terragruntOptions)
			if err != nil {
				return externalDependencies, err
			}

			externalDependency.AssumeAlreadyApplied = alreadyApplied
			allExternalDependencies[externalDependency.Path] = externalDependency
		}
	}

	return allExternalDependencies, nil
}

// Look through the dependencies of the given module and resolve the "external" dependency paths listed in the module's
// config (i.e. those dependencies not in the given list of Terragrunt config canonical file paths). These external
// dependencies are outside of the current working directory, which means they may not be part of the environment the
// user is trying to apply-all or destroy-all. Note that this method will NOT fill in the Dependencies field of the
// TerraformModule struct (see the crosslinkDependencies method for that).
func resolveExternalDependenciesForModule(module *TerraformModule, canonicalTerragruntConfigPaths []string, terragruntOptions *options.TerragruntOptions) (map[string]*TerraformModule, error) {
	if module.Config.Dependencies == nil || len(module.Config.Dependencies.Paths) == 0 {
		return map[string]*TerraformModule{}, nil
	}

	externalTerragruntConfigPaths := []string{}
	for _, dependency := range module.Config.Dependencies.Paths {
		dependencyPath, err := util.CanonicalPath(dependency, module.Path)
		if err != nil {
			return map[string]*TerraformModule{}, err
		}

		terragruntConfigPath := util.JoinPath(dependencyPath, config.DefaultTerragruntConfigPath)
		if !util.ListContainsElement(canonicalTerragruntConfigPaths, terragruntConfigPath) {
			externalTerragruntConfigPaths = append(externalTerragruntConfigPaths, terragruntConfigPath)
		}
	}
	return resolveModules(externalTerragruntConfigPaths, terragruntOptions)
}

// Confirm with the user whether they want Terragrunt to assume the given dependency of the given module is already
// applied. If the user selects "no", then Terragrunt will apply that module as well.
func confirmExternalDependencyAlreadyApplied(module *TerraformModule, dependency *TerraformModule, terragruntOptions *options.TerragruntOptions) (bool, error) {
	prompt := fmt.Sprintf("Module %s depends on module %s, which is an external dependency outside of the current working directory. "+
		"Should Terragrunt skip over this external dependency? Warning, if you say 'no', Terragrunt will make changes in %s as well!",
		module.Path, dependency.Path, dependency.Path)
	return shell.PromptUserForYesNo(prompt, terragruntOptions)
}

// Merge the given external dependencies into the given map of modules if those dependencies aren't already in the
// modules map
func mergeMaps(modules map[string]*TerraformModule, externalDependencies map[string]*TerraformModule) map[string]*TerraformModule {
	out := map[string]*TerraformModule{}

	for key, value := range externalDependencies {
		out[key] = value
	}

	for key, value := range modules {
		out[key] = value
	}

	return out
}

// Go through each module in the given map and cross-link its dependencies to the other modules in that same map. If
// a dependency is referenced that is not in the given map, return an error.
func crosslinkDependencies(moduleMap map[string]*TerraformModule, canonicalTerragruntConfigPaths []string) ([]*TerraformModule, error) {
	modules := []*TerraformModule{}

	for _, module := range moduleMap {
		dependencies, err := getDependenciesForModule(module, moduleMap, canonicalTerragruntConfigPaths)
		if err != nil {
			return modules, err
		}

		module.Dependencies = dependencies
		modules = append(modules, module)
	}

	return modules, nil
}

// Get the list of modules this module depends on
func getDependenciesForModule(module *TerraformModule, moduleMap map[string]*TerraformModule, terragruntConfigPaths []string) ([]*TerraformModule, error) {
	dependencies := []*TerraformModule{}

	if module.Config.Dependencies == nil || len(module.Config.Dependencies.Paths) == 0 {
		return dependencies, nil
	}

	for _, dependencyPath := range module.Config.Dependencies.Paths {
		if module.Path == dependencyPath {
			continue
		}
		dependencyModulePath, err := util.CanonicalPath(dependencyPath, module.Path)
		if err != nil {
			return dependencies, nil
		}

		dependencyModule, foundModule := moduleMap[dependencyModulePath]
		if !foundModule {
			err := UnrecognizedDependency{
				ModulePath:            module.Path,
				DependencyPath:        dependencyPath,
				TerragruntConfigPaths: terragruntConfigPaths,
			}
			return dependencies, errors.WithStackTrace(err)
		}
		dependencies = append(dependencies, dependencyModule)
	}

	return dependencies, nil
}

// Custom error types

type UnrecognizedDependency struct {
	ModulePath            string
	DependencyPath        string
	TerragruntConfigPaths []string
}

func (err UnrecognizedDependency) Error() string {
	return fmt.Sprintf("Module %s specifies %s as a dependency, but that dependency was not one of the ones found while scanning subfolders: %v", err.ModulePath, err.DependencyPath, err.TerragruntConfigPaths)
}
