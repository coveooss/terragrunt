package configstack

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/errors"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/coveooss/terragrunt/v2/util"
)

// TerraformModule represents a single module (i.e. folder with Terraform templates), including the Terragrunt configuration for that
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

// ResolveTerraformModules goes through each of the given Terragrunt configuration files and resolve the module that configuration file represents
// into a TerraformModule struct. Return the list of these TerraformModule structures.
func ResolveTerraformModules(terragruntConfigPaths []string, terragruntOptions *options.TerragruntOptions) ([]*TerraformModule, error) {
	canonicalTerragruntConfigPaths, err := util.CanonicalPaths(terragruntConfigPaths, ".")
	if err != nil {
		return []*TerraformModule{}, err
	}

	modules, err := resolveModules(canonicalTerragruntConfigPaths, terragruntOptions, false)
	if err != nil {
		return []*TerraformModule{}, err
	}

	// We remove any path that are not in the resolved module
	if terragruntOptions.CheckSourceFolders {
		canonicalTerragruntConfigPaths = func() []string {
			filtered := make([]string, 0, len(canonicalTerragruntConfigPaths))
			for _, path := range canonicalTerragruntConfigPaths {
				if modules[path] != nil {
					filtered = append(filtered, path)
				}
			}
			return filtered
		}()
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
//
// resolveExternal is used to exclude modules that don't contain terraform files. This is used to avoid requirements of
// adding terragrunt.ignore when a parent folder doesn't have terraform files to deploy by itself.
func resolveModules(canonicalTerragruntConfigPaths []string, terragruntOptions *options.TerragruntOptions, resolveExternal bool) (map[string]*TerraformModule, error) {
	moduleMap := map[string]*TerraformModule{}

	for _, terragruntConfigPath := range canonicalTerragruntConfigPaths {
		if module, tfFiles, err := resolveTerraformModule(terragruntConfigPath, terragruntOptions); err == nil {
			if resolveExternal && module != nil || tfFiles {
				moduleMap[module.Path] = module
			}
		} else {
			return moduleMap, err
		}
	}

	return moduleMap, nil
}

// Create a TerraformModule struct for the Terraform module specified by the given Terragrunt configuration file path.
// Note that this method will NOT fill in the Dependencies field of the TerraformModule struct (see the
// crosslinkDependencies method for that).
func resolveTerraformModule(terragruntConfigPath string, terragruntOptions *options.TerragruntOptions) (module *TerraformModule, tfFiles bool, err error) {
	modulePath, err := util.CanonicalPath(filepath.Dir(terragruntConfigPath), ".")
	if err != nil {
		return
	}

	opts := terragruntOptions.Clone(terragruntConfigPath)
	_, terragruntConfig, err := config.ParseConfigFile(opts, config.IncludeConfig{Path: terragruntConfigPath})
	if err != nil {
		return
	}

	// Fix for https://github.com/gruntwork-io/terragrunt/issues/208
	matches, err := utils.FindFiles(filepath.Dir(terragruntConfigPath), false, false, options.TerraformFilesTemplates...)
	if err != nil {
		return
	}
	if matches == nil {
		if terragruntConfig.Terraform == nil || terragruntConfig.Terraform.Source == "" {
			terragruntOptions.Logger.Debugf("Module %s does not have an associated terraform configuration and will be skipped.", filepath.Dir(terragruntConfigPath))
			return
		}
		if terragruntOptions.CheckSourceFolders {
			sourcePath := terragruntConfig.Terraform.Source
			if !filepath.IsAbs(sourcePath) {
				sourcePath, _ = util.CanonicalPath(sourcePath, filepath.Dir(terragruntConfigPath))
			}
			matches, err = utils.FindFiles(sourcePath, false, false, options.TerraformFilesTemplates...)
		}
	}

	tfFiles = len(matches) > 0 || !terragruntOptions.CheckSourceFolders
	module = &TerraformModule{Path: modulePath, Config: *terragruntConfig, TerragruntOptions: opts}
	return
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
func resolveExternalDependenciesForModule(module *TerraformModule, canonicalTerragruntConfigPaths []string, terragruntOptions *options.TerragruntOptions) (result map[string]*TerraformModule, err error) {
	result = make(map[string]*TerraformModule)
	if module.Config.Dependencies == nil || len(module.Config.Dependencies.Paths) == 0 {
		return
	}

	externalTerragruntConfigPaths := []string{}
	for _, dependency := range module.Config.Dependencies.Paths {
		var dependencyPath string
		if dependencyPath, err = util.CanonicalPath(dependency, module.Path); err != nil {
			return
		}

		var configs []string
		if terragruntConfigPath, exists := terragruntOptions.ConfigPath(dependencyPath); exists {
			configs = append(configs, terragruntConfigPath)
		} else if util.FileExists(dependencyPath) {
			if configs, err = terragruntOptions.FindConfigFilesInPath(dependencyPath); err != nil {
				return
			}
		}
		for _, config := range configs {
			if !util.ListContainsElement(canonicalTerragruntConfigPaths, config) {
				externalTerragruntConfigPaths = append(externalTerragruntConfigPaths, config)
			}
		}
	}
	return resolveModules(externalTerragruntConfigPaths, terragruntOptions, true)
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
	dependencies := make([]*TerraformModule, 0, len(moduleMap))
	dependenciesPaths := make([]string, 0, len(moduleMap))

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

		if dependencyModule, foundModule := moduleMap[dependencyModulePath]; foundModule {
			if !util.ListContainsElement(dependenciesPaths, dependencyModulePath) {
				// We avoid adding the same module dependency more than once
				dependenciesPaths = append(dependenciesPaths, dependencyModulePath)
				dependencies = append(dependencies, dependencyModule)
			}
		} else {
			var foundModules []*TerraformModule
			// The dependency may be a parent folder
			for _, key := range collections.AsDictionary(moduleMap).KeysAsString() {
				if key.HasPrefix(dependencyModulePath + "/") {
					foundModule = true
					if !util.ListContainsElement(dependenciesPaths, key.Str()) {
						// We avoid adding the same module dependency more than once
						dependenciesPaths = append(dependenciesPaths, key.Str())
						foundModules = append(foundModules, moduleMap[key.Str()])
					}
				}
			}
			if !foundModule && !module.AssumeAlreadyApplied {
				err := UnrecognizedDependency{
					ModulePath:            module.Path,
					DependencyPath:        dependencyPath,
					TerragruntConfigPaths: terragruntConfigPaths,
				}
				return dependencies, errors.WithStackTrace(err)
			}
			dependencies = append(dependencies, foundModules...)
		}
	}

	return dependencies, nil
}

// Custom error types

// UnrecognizedDependency describes error when a dependency cannot be resolved
type UnrecognizedDependency struct {
	ModulePath            string
	DependencyPath        string
	TerragruntConfigPaths []string
}

func (err UnrecognizedDependency) Error() string {
	return fmt.Sprintf("Module %s specifies %s as a dependency, but that dependency was not one of the ones found while scanning subfolders: %v", err.ModulePath, err.DependencyPath, err.TerragruntConfigPaths)
}
