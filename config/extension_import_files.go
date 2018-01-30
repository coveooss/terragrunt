package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/aws_helper"
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
	Prefix            string          `hcl:"prefix"`
}

// CopyAndRename is a structure used by ImportFiles to rename the imported files
type copyAndRename struct {
	Source string `hcl:"source"`
	Target string `hcl:"target"`
}

func (item ImportFiles) itemType() (result string) { return ImportFilesList{}.argName() }

func (item *ImportFiles) normalize() {
	if item.Required == nil {
		def := true
		item.Required = &def
	}
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

	target, _ := filepath.Rel(item.options().WorkingDir, item.Target)
	for _, file := range item.Files {
		target := filepath.Join(target, fmt.Sprintf("%s%s", item.Prefix, filepath.Base(file)))
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
	logger := item.logger()

	if !item.enabled() {
		logger.Debugf("Import file %s skipped, executed only on %v", item.Name, item.OS)
		return
	}

	// If no folders are specified, we only copy elements to the working folder
	if len(folders) == 0 {
		folders = []interface{}{item.options().WorkingDir}
	}

	var sourceFolder, sourceFolderPrefix string
	if item.Source != "" {
		sourceFolder, err = util.GetSource(item.Source, filepath.Dir(item.config().Path), logger)
		if err != nil {
			if *item.Required {
				return
			}
			logger.Warningf("%s: %s doesn't exist", item.Name, item.Source)
		}
		sourceFolderPrefix = fmt.Sprintf("%s%c", sourceFolder, filepath.Separator)
	}

	for _, folder := range folders {
		var messages []string

		if sourceFolder != "" {
			messages = append(messages, fmt.Sprintf("from %s", item.Source))
		}
		folder := folder.(string)
		isModule := item.options().WorkingDir != folder
		if isModule {
			if !item.ImportIntoModules {
				// We skip import in the folder if the item doesn't require to be applied on modules
				continue
			}
		}

		// Check if the item has a specific target folder
		importerTarget := folder
		if item.Target != "" {
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
		relativeTarget := util.GetPathRelativeToMax(importerTarget, item.options().WorkingDir, 2)
		if relativeTarget != "" && relativeTarget != "." {
			messages = append(messages, fmt.Sprintf("to %s", relativeTarget))
		}

		if item.Prefix != "" {
			messages = append(messages, fmt.Sprintf("prefixed by %s", item.Prefix))
		}
		contextMessage := fmt.Sprintf(" %s", strings.Join(messages, " "))

		// Local copy function used by both type of file copy
		copy := func(source, target string) error {
			target = filepath.Join(importerTarget, target)

			folder, file := filepath.Split(target)
			target = filepath.Join(folder, item.Prefix+file)
			os.MkdirAll(folder, os.ModePerm)
			if err := util.CopyFile(source, target); err != nil {
				return err
			}
			if item.FileMode != nil {
				return os.Chmod(target, os.FileMode(*item.FileMode))
			}
			return nil
		}

		type fileCopy struct {
			source, target string
		}
		var sourceFiles []fileCopy

		if len(item.Files) == 0 {
			item.Files = []string{"*"}
		}

		var filePatterns, pathPatterns []string
		for _, pattern := range item.Files {
			var newFiles []fileCopy
			if filepath.IsAbs(pattern) {
				var files []string
				files, err = filepath.Glob(pattern)
				if err != nil {
					err = fmt.Errorf("Invalid pattern in %s", pattern)
					return
				}
				if *item.Required && len(files) == 0 {
					err = fmt.Errorf("Unable to import required file %s", pattern)
					return
				}

				for _, file := range files {
					if err := ensureIsFile(file); err != nil {
						logger.Warningf("%s(%s): %v", item.itemType(), item.id(), err)
					} else {
						newFiles = append(newFiles, fileCopy{source: file})
					}
				}
			} else {
				if strings.Contains(pattern, string(filepath.Separator)) {
					pathPatterns = append(pathPatterns, pattern)
				} else {
					filePatterns = append(filePatterns, pattern)
				}
			}

			_ = filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
				if ensureIsFile(path) != nil || strings.HasSuffix(path, aws_helper.CacheFile) {
					return nil
				}
				relName := strings.TrimPrefix(path, sourceFolder)[1:]
				for _, pattern := range filePatterns {
					if match, _ := filepath.Match(pattern, filepath.Base(relName)); match {
						newFiles = append(newFiles, fileCopy{source: path})
					}
				}
				for _, pattern := range pathPatterns {
					if match, _ := filepath.Match(pattern, relName); match {
						newFiles = append(newFiles, fileCopy{source: path})
					}
				}
				return nil
			})

			if *item.Required && len(newFiles) == 0 {
				err = fmt.Errorf("Unable to import required file %s", strings.Join(item.Files, ", "))
				return
			}

			for i := range newFiles {
				var target string
				if item.Target != "" || sourceFolder == "" {
					newFiles[i].target = filepath.Base(newFiles[i].source)
				} else {
					target = strings.TrimPrefix(newFiles[i].source, sourceFolderPrefix)
					folder, base := filepath.Split(target)
					newFiles[i].target = filepath.Join(folder, base)
				}
			}
			sourceFiles = append(sourceFiles, newFiles...)

			if len(newFiles) == 1 {
				logger.Infof("Import file %s%s", newFiles[0].target, contextMessage)
			} else {
				copiedFiles := make([]string, len(newFiles))
				for i := range newFiles {
					copiedFiles[i] = newFiles[i].target
				}
				logger.Infof("Import file %s: %s%s", pattern, strings.Join(copiedFiles, ", "), contextMessage)
			}
		}

		for _, source := range sourceFiles {
			if util.FileExists(source.source) {
				if err = copy(source.source, source.target); err != nil {
					return
				}
			} else if *item.Required {
				err = fmt.Errorf("Unable to import required file %s", source)
				return
			} else if !isModule {
				logger.Debugf("Skipping copy of %s, the source is not found", source)
			}
		}

		for _, source := range item.CopyAndRename {
			if util.FileExists(source.Source) {
				logger.Infof("Import file %s to %s%s", filepath.Base(source.Source), source.Target, contextMessage)
				if err = copy(source.Source, source.Target); err != nil {
					return
				}
			} else if *item.Required {
				err = fmt.Errorf("Unable to import required file %s", source.Source)
				return
			} else if !isModule {
				logger.Debugf("Skipping copy of %s, the source is not found", source)
			}
		}
	}
	return
}

func ensureIsFile(file string) error {
	if stat, err := os.Stat(file); err != nil {
		return err
	} else if stat.IsDir() {
		return fmt.Errorf("Folder ignored %s", file)
	}
	return nil
}

// ----------------------- ImportFilesList -----------------------

//go:generate genny -in=extension_base_list.go -out=generated_import_files.go gen "GenericItem=ImportFiles"
func (list ImportFilesList) argName() string       { return "import_files" }
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
