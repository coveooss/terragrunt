//lint:file-ignore U1000 Ignore all unused code, it's generated

package config

import (
	"fmt"
	"strings"

	"github.com/coveooss/multilogger/errors"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/util"
)

// ImportVariables is a configuration that allows defintion of variables that will be added to
// the current execution. Variables could be defined either by loading files (required or optional)
// or defining vairables directly. It is also possible to define global environment variables.
type ImportVariables struct {
	TerragruntExtensionBase `hcl:",remain"`

	Vars             []string          `hcl:"vars,optional"`
	RequiredVarFiles []string          `hcl:"required_var_files,optional"`
	OptionalVarFiles []string          `hcl:"optional_var_files,optional"`
	NoTemplating     bool              `hcl:"no_templating_in_files,optional"`
	Sources          []string          `hcl:"sources,optional"`
	NestedObjects    []string          `hcl:"nested_under,optional"`
	EnvVars          map[string]string `hcl:"env_vars,optional"`
	OnCommands       []string          `hcl:"on_commands,optional"`
	SourceFileRegex  string            `hcl:"source_file_regex,optional"`
}

func (item ImportVariables) itemType() (result string) {
	return ImportVariablesList{}.argName()
}

func (item ImportVariables) help() (result string) {
	if item.Description != "" {
		result += fmt.Sprintf("\n%s\n", item.Description)
	}
	if item.OnCommands != nil {
		result += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(item.OnCommands, ", "))
	}

	return
}

func (item *ImportVariables) normalize() {
	if len(item.NestedObjects) == 0 {
		// By default, we load the variables at the root
		item.NestedObjects = []string{""}
	}
}

func (item *ImportVariables) loadVariablesFromFile(file string) error {
	item.logger().Debug("Importing ", file)
	vars, err := item.options().LoadVariablesFromTemplateFile(file, !item.NoTemplating)
	if err != nil {
		return err
	}
	item.loadVariables(vars, options.VarFile)
	return nil
}

func (item *ImportVariables) loadVariables(newVariables map[string]interface{}, source options.VariableSource) {
	for key, value := range newVariables {
		// Simplify the reference to variables in case the key is repeated (ex: project.project.value can be directly accessed with project.value)
		if map1, isMap := value.(map[string]interface{}); isMap {
			if map2, isMap := map1[key].(map[string]interface{}); isMap {
				for mapKey, mapValue := range map2 {
					if _, exist := map1[mapKey]; !exist {
						map1[mapKey] = mapValue
					}
				}
			}
		}
	}
	for _, nested := range item.NestedObjects {
		imported := newVariables
		if nested != "" {
			imported = map[string]interface{}{nested: imported}
		}
		item.options().ImportVariablesMap(imported, source)
	}
}

// ----------------------- ImportVariablesList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_import_variables.go gen "GenericItem=ImportVariables"
func (list ImportVariablesList) argName() string           { return "import_variables" }
func (list ImportVariablesList) sort() ImportVariablesList { return list }

// Merge elements from an imported list to the current list
func (list *ImportVariablesList) Merge(imported ImportVariablesList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

// Import actually process the variables importers to load and define all variables in the current context
func (list ImportVariablesList) Import() (err error) {
	if len(list) == 0 {
		return nil
	}

	config := IImportVariables(&list[0]).config()
	terragruntOptions := config.options

	for _, item := range list.Enabled() {
		if len(item.OnCommands) > 0 && !util.ListContainsElement(item.OnCommands, item.options().Env[options.EnvCommand]) {
			// The current command is not in the list of command on which the import should be applied
			return
		}
		item.logger().Debugf("Processing import variables statement %s", item.id())

		// Set environment variables
		for key, value := range item.EnvVars {
			terragruntOptions.Env[key] = value
		}

		folders := []string{terragruntOptions.WorkingDir}
		var folderErrors errors.Array
		if len(item.Sources) > 0 {
			folders = make([]string, 0, len(item.Sources))
			for _, source := range item.Sources {
				if source == "" {
					source = terragruntOptions.WorkingDir
				}
				source, err := config.GetSourceFolder(item.Name, source, true, item.SourceFileRegex)
				if err != nil {
					folderErrors = append(folderErrors, err)
					continue
				}
				folders = append(folders, source)
			}
		}
		if len(folders) == 0 {
			if len(item.RequiredVarFiles) > 0 {
				return folderErrors
			}
			continue
		}

		// We first process all the -var because they have precedence over -var-file
		keyFunc := func(key string) string { return strings.Split(key, "=")[0] }
		varList := util.RemoveDuplicatesFromList(item.Vars, true, keyFunc)
		for _, varDef := range varList {
			var (
				key   string
				value interface{}
			)

			if !strings.Contains(varDef, "=") {
				key = varDef
				value = nil
			} else {
				if key, value, err = util.SplitEnvVariable(varDef); err != nil {
					terragruntOptions.Logger.Warningf("-var ignored in %v: %v", item.Name, err)
					continue
				}
			}
			if util.ListContainsElement(terragruntOptions.VariablesExplicitlyProvided(), key) {
				continue
			}
			item.loadVariables(map[string]interface{}{key: value}, options.VarParameter)
		}

		// Process RequiredVarFiles
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.RequiredVarFiles) {
			files := config.globFiles(pattern, true, folders...)
			if len(files) == 0 {
				return fmt.Errorf("%s: No file matches %s", item.name(), pattern)
			}
			for _, file := range files {
				if err = item.loadVariablesFromFile(file); err != nil {
					return
				}
			}
		}

		// Processes OptionalVarFiles
		for _, pattern := range util.RemoveDuplicatesFromListKeepLast(item.OptionalVarFiles) {
			for _, file := range config.globFiles(pattern, true, folders...) {
				if util.FileExists(file) {
					if err = item.loadVariablesFromFile(file); err != nil {
						return
					}
				} else {
					terragruntOptions.Logger.Tracef("Skipping var-file %s as it does not exist", file)
				}
			}
		}
	}

	return
}
