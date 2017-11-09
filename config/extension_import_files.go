package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/util"
)

// ImportFiles is a configuration of files that must be imported from another directory to the terraform directory
// prior executing terraform commands
type ImportFiles struct {
	TerragruntExtensionBase `hcl:",squash"`
	Source                  string
	Files                   []string
	CopyAndRename           []copyAndRename `hcl:"copy_and_remove"`
	Required                *bool
	ImportIntoModules       bool `hcl:"import_into_module"`
	FileMode                *int `hcl:"file_mode"`
	Target                  string
	Prefix                  *string
}

// CopyAndRename is a structure used by ImportFiles to rename the imported files
type copyAndRename struct {
	Source string `hcl:"source"`
	Target string `hcl:"target"`
}

func (importer ImportFiles) String() string {
	files := importer.Files

	for _, copy := range importer.CopyAndRename {
		files = append(files, fmt.Sprintf("%s → %s", copy.Source, copy.Target))
	}

	return fmt.Sprintf("ImportFiles %s %s required=%v modules=%v : %s",
		importer.Name, importer.Source,
		importer.Required, importer.ImportIntoModules,
		strings.Join(files, ", "))
}

func (importer *ImportFiles) run(folders []string) error {
	logger := importer.Logger()

	if importer.Prefix == nil {
		prefix := importer.Name + "_"
		importer.Prefix = &prefix
	}

	if importer.Required == nil {
		def := true
		importer.Required = &def
	}

	var sourceFolder string
	if importer.Source != "" {
		var err error
		sourceFolder, err = util.GetSource(importer.Source, logger)
		if err != nil {
			if *importer.Required {
				return err
			}
			logger.Warningf("%s: %s doesn't exist", importer.Name, importer.Source)
		}
	}

	for _, folder := range folders {
		folderName := "Temporary folder"
		isModule := importer.Options().WorkingDir != folder
		if isModule {
			if !importer.ImportIntoModules {
				// We skip import in the folder if the importer doesn't require to be applied on modules
				continue
			}
			folderName = filepath.Base(folder)
		}

		// Check if the importer has a specific target folder
		importerTarget := folder
		if importer.Target != "" {
			folderName = importer.Target
			if filepath.IsAbs(importer.Target) {
				importerTarget = importer.Target
			} else {
				importerTarget = filepath.Join(folder, importer.Target)
			}
			err := os.MkdirAll(importerTarget, 0755)
			if err != nil {
				return err
			}
		}

		// Local copy function used by both type of file copy
		copy := func(source, target string) error {
			target = filepath.Join(importerTarget, target)
			if err := util.CopyFile(source, target); err != nil {
				return err
			}
			if importer.FileMode != nil {
				return os.Chmod(target, os.FileMode(*importer.FileMode))
			}
			return nil
		}

		var sourceFiles []string
		for _, pattern := range importer.Files {
			if sourceFolder != "" {
				pattern = filepath.Join(sourceFolder, pattern)
			} else if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(importer.Options().WorkingDir, pattern)
			}
			files, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("Invalid pattern %s", filepath.Base(pattern))
			}

			if len(files) > 0 {
				fileBases := make([]string, len(files))
				for i, file := range files {
					fileBases[i] = filepath.Base(file)
				}
				logger.Infof("%s: Copy %s to %s", importer.Name, strings.Join(fileBases, ", "), folderName)
			} else if *importer.Required {
				return fmt.Errorf("Unable to import required file %s", pattern)
			}
			sourceFiles = append(sourceFiles, files...)
		}

		for _, source := range sourceFiles {
			if util.FileExists(source) {
				if err := copy(source, *importer.Prefix+filepath.Base(source)); err != nil {
					return err
				}
			} else if *importer.Required {
				return fmt.Errorf("Unable to import required file %s", source)
			} else if !isModule {
				logger.Debugf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}

		for _, source := range importer.CopyAndRename {
			if util.FileExists(source.Source) {
				logger.Infof("Copy file %s to %s/%v", filepath.Base(source.Source), folderName, source.Target)
				if err := copy(source.Source, source.Target); err != nil {
					return err
				}
			} else if *importer.Required {
				return fmt.Errorf("Unable to import required file %s", source.Source)
			} else if !isModule {
				logger.Debugf("Skipping copy of %s to %s, the source is not found", source, folderName)
			}
		}
	}
	return nil
}

// ----------------------- ImportFilesList -----------------------

// ImportFilesList represents an array of ImportFiles objects
type ImportFilesList []ImportFiles

// Help returns the help string for an array of Hook objects
func (importers ImportFilesList) Help(listOnly bool) string {
	var result string

	for _, importer := range importers {
		result += fmt.Sprintf("\n%s", item(importer.Name))
		if listOnly {
			continue
		}
		result += fmt.Sprintln()
		if importer.Description != "" {
			result += fmt.Sprintf("\n%s\n", importer.Description)
		}
		if importer.Source != "" {
			result += fmt.Sprintf("\nFrom %s:\n", importer.Source)
		} else {
			result += fmt.Sprint("\nFile(s):\n")
		}

		prefix := importer.Name + "_"
		if importer.Prefix != nil {
			prefix = *importer.Prefix
		}

		target, _ := filepath.Rel(importer.Options().WorkingDir, importer.Target)
		for _, file := range importer.Files {
			target := filepath.Join(target, fmt.Sprintf("%s%s", prefix, filepath.Base(file)))
			if strings.Contains(file, "/terragrunt-cache/") {
				file = filepath.Base(file)
			}
			result += fmt.Sprintf("   %s → %s\n", file, target)
		}

		required := true
		if importer.Required != nil {
			required = *importer.Required
		}

		attributes := []string{fmt.Sprintf("Required = %v", required)}
		if importer.ImportIntoModules {
			attributes = append(attributes, "Import into modules")
		}
		if importer.FileMode != nil {
			attributes = append(attributes, fmt.Sprintf("File mode = %#o", *importer.FileMode))
		}
		result += fmt.Sprintf("\n%s\n", strings.Join(attributes, ", "))
	}
	return result
}

// Run executes importers configuration on main folder
func (importers ImportFilesList) Run() error {
	return importers.run()
}

// RunOnModules executes importers configuration on module folders
func (importers ImportFilesList) RunOnModules() error {
	modules, err := importers.getModulesFolders()
	if err != nil {
		return err
	}

	return importers.run(modules...)
}

func (importers ImportFilesList) run(folders ...string) error {
	if len(importers) == 0 {
		return nil
	}

	if len(folders) == 0 {
		folders = []string{importers[0].Options().WorkingDir}
	}
	logger := importers[0].Logger()

	for _, importer := range importers {
		if !importer.Enabled() {
			logger.Debugf("Importer %s skipped, executed only on %v", importer.Name, importer.OS)
			continue
		}

		if err := importer.run(folders); err != nil {
			return err
		}
	}
	return nil
}

func (importers ImportFilesList) getModulesFolders() ([]string, error) {
	if len(importers) == 0 {
		return nil, nil
	}

	options := importers[0].Options()
	logger := importers[0].Logger()

	modules, _ := filepath.Glob(filepath.Join(options.WorkingDir, ".terraform", "modules", "*"))
	folders := make(map[string]int)
	for _, module := range modules {
		stat, err := os.Stat(module)
		if err != nil {
			return nil, err
		}
		if !stat.IsDir() {
			logger.Warning("Unexpected file in .terraform/modules:", module)
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

	keys := make([]string, 0, len(folders))
	for key := range folders {
		keys = append(keys, key)
	}
	return keys, nil
}
