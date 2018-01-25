package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

// ImportFiles is a configuration of files that must be imported from another directory to the terraform directory
// prior executing terraform commands
type ImportFiles struct {
	TerragruntExtensionBase `hcl:",squash"`

	Source            string          `hcl:"source"`
	Files             []string        `hcl:"files"`
	CopyAndRename     []copyAndRename `hcl:"copy_and_remove"`
	Required          *bool           `hcl:"required,omitempty"`
	ImportIntoModules bool            `hcl:"import_into_modules"`
	FileMode          *int            `hcl:"file_mode"`
	Target            string          `hcl:"target"`
	Prefix            *string         `hcl:"prefix"`
}

// CopyAndRename is a structure used by ImportFiles to rename the imported files
type copyAndRename struct {
	Source string `hcl:"source"`
	Target string `hcl:"target"`
}

func (item ImportFiles) help() (result string) {
	if item.Description != "" {
		result += fmt.Sprintf("\n%s\n", item.Description)
	}
	if item.Source != "" {
		result += fmt.Sprintf("\nFrom %s:\n", item.Source)
	} else {
		result += fmt.Sprint("\nFile(s):\n")
	}

	prefix := item.Name + "_"
	if item.Prefix != nil {
		prefix = *item.Prefix
	}

	target, _ := filepath.Rel(item.options().WorkingDir, item.Target)
	for _, file := range item.Files {
		target := filepath.Join(target, fmt.Sprintf("%s%s", prefix, filepath.Base(file)))
		if strings.Contains(file, "/terragrunt-cache/") {
			file = filepath.Base(file)
		}
		result += fmt.Sprintf("   %s → %s\n", file, target)
	}

	required := true
	if item.Required != nil {
		required = *item.Required
	}

	attributes := []string{fmt.Sprintf("Required = %v", required)}
	if item.ImportIntoModules {
		attributes = append(attributes, "Import into modules")
	}
	if item.FileMode != nil {
		attributes = append(attributes, fmt.Sprintf("File mode = %#o", *item.FileMode))
	}
	result += fmt.Sprintf("\n%s\n", strings.Join(attributes, ", "))
	return
}

func (item *ImportFiles) run(folders ...interface{}) (result []interface{}, err error) {
	if len(folders) == 0 {
		folders = []interface{}{item.options().WorkingDir}
	}
	logger := item.logger()

	if item.Prefix == nil {
		prefix := item.Name + "_"
		item.Prefix = &prefix
	}

	if item.Required == nil {
		def := true
		item.Required = &def
	}

	var sourceFolder string
	if item.Source != "" {
		sourceFolder, err = util.GetSource(item.Source, logger)
		if err != nil {
			if *item.Required {
				return
			}
			logger.Warningf("%s: %s doesn't exist", item.Name, item.Source)
		}
	}

	for _, folder := range folders {
		folder := folder.(string)
		folderName := "Temporary folder"
		isModule := item.options().WorkingDir != folder
		if isModule {
			if !item.ImportIntoModules {
				// We skip import in the folder if the item doesn't require to be applied on modules
				continue
			}
			folderName = filepath.Base(folder)
		}

		// Check if the item has a specific target folder
		importerTarget := folder
		if item.Target != "" {
			folderName = item.Target
			if filepath.IsAbs(item.Target) {
				importerTarget = item.Target
			} else {
				importerTarget = filepath.Join(folder, item.Target)
			}
			err = os.MkdirAll(importerTarget, 0755)
			if err != nil {
				return
			}
		}

		// Local copy function used by both type of file copy
		copy := func(source, target string) error {
			target = filepath.Join(importerTarget, target)
			if err := util.CopyFile(source, target); err != nil {
				return err
			}
			if item.FileMode != nil {
				return os.Chmod(target, os.FileMode(*item.FileMode))
			}
			return nil
		}

		var sourceFiles []string
		for _, pattern := range item.Files {
			if sourceFolder != "" {
				pattern = filepath.Join(sourceFolder, pattern)
			} else if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(item.options().WorkingDir, pattern)
			}
			var files []string
			if files, err = filepath.Glob(pattern); err != nil {
				err = fmt.Errorf("Invalid pattern %s", filepath.Base(pattern))
				return
			}

			if len(files) > 0 {
				fileBases := make([]string, len(files))
				for i, file := range files {
					fileBases[i] = filepath.Base(file)
				}
				logger.Infof("%s: Copy %s to %s", item.Name, strings.Join(fileBases, ", "), folderName)
			} else if *item.Required {
				err = fmt.Errorf("Unable to import required file %s", pattern)
				return
			}
			sourceFiles = append(sourceFiles, files...)
		}

		for _, source := range sourceFiles {
			if util.FileExists(source) {
				if err = copy(source, *item.Prefix+filepath.Base(source)); err != nil {
					return
				}
			} else if *item.Required {
				err = fmt.Errorf("Unable to import required file %s", source)
				return
			} else if !isModule {
				logger.Debugf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}

		for _, source := range item.CopyAndRename {
			if util.FileExists(source.Source) {
				logger.Infof("Copy file %s to %s/%v", filepath.Base(source.Source), folderName, source.Target)
				if err = copy(source.Source, source.Target); err != nil {
					return
				}
			} else if *item.Required {
				err = fmt.Errorf("Unable to import required file %s", source.Source)
				return
			} else if !isModule {
				logger.Debugf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}
	}
	return
}

// ----------------------- ImportFilesList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_import_files.go gen "GenericItem=ImportFiles"
func (list *ImportFilesList) argName() string      { return "import_files" }
func (list ImportFilesList) sort() ImportFilesList { return list }

// Merge elements from an imported list to the current list
func (list *ImportFilesList) Merge(imported ImportFilesList) {
	list.merge(imported, mergeModePrepend, list.argName())
}

// RunOnModules executes list configuration on module folders
func (list ImportFilesList) RunOnModules(terragruntOptions *options.TerragruntOptions) (result interface{}, err error) {
	if len(list) == 0 {
		return
	}

	modules, _ := filepath.Glob(filepath.Join(terragruntOptions.WorkingDir, ".terraform", "modules", "*"))
	folders := make(map[string]int)
	for _, module := range modules {
		stat, err := os.Stat(module)
		if err != nil {
			return nil, err
		}
		if !stat.IsDir() {
			continue
		}

		stat, _ = os.Lstat(module)
		if !stat.IsDir() {
			link, err := os.Readlink(module)
			if err != nil {
				return nil, err
			}
			module = link
		}
		folders[module] = folders[module] + 1
	}
	if len(folders) == 0 {
		return
	}

	keys := make([]interface{}, 0, len(folders))
	for key := range folders {
		keys = append(keys, key)
	}

	return list.Run(keys...)
}
