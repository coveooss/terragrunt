package util

import (
	"bytes"
	"fmt"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/hashicorp/hcl"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Return true if the given file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Return the canonical version of the given path, relative to the given base path. That is, if the given path is a
// relative path, assume it is relative to the given base path. A canonical path is an absolute path with all relative
// components (e.g. "../") fully resolved, which makes it safe to compare paths as strings.
func CanonicalPath(path string, basePath string) (string, error) {
	if !filepath.IsAbs(path) {
		path = JoinPath(basePath, path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return CleanPath(absPath), nil
}

// Return the canonical version of the given paths, relative to the given base path. That is, if a given path is a
// relative path, assume it is relative to the given base path. A canonical path is an absolute path with all relative
// components (e.g. "../") fully resolved, which makes it safe to compare paths as strings.
func CanonicalPaths(paths []string, basePath string) ([]string, error) {
	canonicalPaths := []string{}

	for _, path := range paths {
		canonicalPath, err := CanonicalPath(path, basePath)
		if err != nil {
			return canonicalPaths, err
		}
		canonicalPaths = append(canonicalPaths, canonicalPath)
	}

	return canonicalPaths, nil
}

// Delete the given list of files. Note: this function ONLY deletes files and will return an error if you pass in a
// folder path.
func DeleteFiles(files []string) error {
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return errors.WithStackTrace(err)
		}
	}
	return nil
}

// Returns true if the given regex can be found in any of the files matched by the given glob
func Grep(regex *regexp.Regexp, glob string) (bool, error) {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return false, errors.WithStackTrace(err)
	}

	for _, match := range matches {
		bytes, err := ioutil.ReadFile(match)
		if err != nil {
			return false, errors.WithStackTrace(err)
		}

		if regex.Match(bytes) {
			return true, nil
		}
	}

	return false, nil
}

// Return the relative path you would have to take to get from basePath to path
func GetPathRelativeTo(path string, basePath string) (string, error) {
	if path == "" {
		path = "."
	}
	if basePath == "" {
		basePath = "."
	}

	inputFolderAbs, err := filepath.Abs(basePath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	fileAbs, err := filepath.Abs(path)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	relPath, err := filepath.Rel(inputFolderAbs, fileAbs)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return filepath.ToSlash(relPath), nil
}

// Return the contents of the file at the given path as a string
func ReadFileAsString(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.WithStackTraceAndPrefix(err, "Error reading file at path %s", path)
	}

	return string(bytes), nil
}

// ReadFileAsStringFromSource returns the contents of the file at the given path
// from an external source (github, s3, etc.) as a string
// It uses terraform to execute its command
func ReadFileAsStringFromSource(source, path string, terraform string) (localFile, content string, err error) {
	cacheDir := filepath.Join(os.TempDir(), "terragrunt-cache", EncodeBase64Sha1(source))
	sharedMutex.Lock()
	defer sharedMutex.Unlock()

	if _, ok := sharedContent[cacheDir]; !ok {
		log := CreateLogger("Copy source")

		cmd := exec.Command(terraform, "init", "-no-color", source, cacheDir)
		cmd.Stdin = os.Stdin
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err = cmd.Run()
		if err != nil {
			log.Error(out.String())
			return
		}
		log.Info(out.String())
		sharedContent[cacheDir] = true
	}
	localFile = filepath.Join(cacheDir, path)
	content, err = ReadFileAsString(localFile)
	return
}

var sharedMutex sync.Mutex
var sharedContent = map[string]bool{}

// FlattenHCL - Convert array of map to single map if there is only one element in the array
// By default, the hcl.Unmarshal returns array of map even if there is only a single map in the definition
func FlattenHCL(source map[string]interface{}) map[string]interface{} {
	for key, value := range source {
		switch value := value.(type) {
		case []map[string]interface{}:
			switch len(value) {
			case 1:
				source[key] = FlattenHCL(value[0])
			default:
				for i, subMap := range value {
					value[i] = FlattenHCL(subMap)
				}
			}
		}
	}
	return source
}

// Return a map of the variables defined in the tfvars file
func LoadDefaultValues(files []string) (map[string]interface{}, error) {
	content := map[string]interface{}{}

	for _, file := range files {
		bytes, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}

		if err = hcl.Unmarshal(bytes, &content); err != nil {
			_, file = filepath.Split(file)
			return nil, fmt.Errorf("%v %v", file, err)
		}
	}

	if variables := content["variable"]; variables != nil {
		switch variables := variables.(type) {
		case []map[string]interface{}:
			result := map[string]interface{}{}
			for _, value := range variables {
				for name, value := range value {
					value := value.([]map[string]interface{})[0]

					if value := value["default"]; value != nil {
						result[name] = value
					}
				}
			}
			return result, nil
		}
	}

	return nil, nil
}

// Return a map of the variables defined in the tfvars file
func LoadTfVars(path string) (map[string]interface{}, error) {
	variables := map[string]interface{}{}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return variables, err
	}

	err = hcl.Unmarshal(bytes, &variables)
	return FlattenHCL(variables), err
}

// Copy the files and folders within the source folder into the destination folder
func CopyFolderContents(source string, destination string) error {
	files, err := ioutil.ReadDir(source)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	for _, file := range files {
		src := JoinPath(source, file.Name())
		dest := JoinPath(destination, file.Name())

		if file.IsDir() {
			if err := os.MkdirAll(dest, file.Mode()); err != nil {
				return errors.WithStackTrace(err)
			}

			if err := CopyFolderContents(src, dest); err != nil {
				return err
			}
		} else {
			if err := CopyFile(src, dest); err != nil {
				return err
			}
		}
	}

	return nil
}

// Copy a file from source to destination
func CopyFile(source string, destination string) error {
	contents, err := ioutil.ReadFile(source)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	return WriteFileWithSamePermissions(source, destination, contents)
}

// Write a file to the given destination with the given contents using the same permissions as the file at source
func WriteFileWithSamePermissions(source string, destination string, contents []byte) error {
	fileInfo, err := os.Stat(source)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	return ioutil.WriteFile(destination, contents, fileInfo.Mode())
}

// Windows systems use \ as the path separator *nix uses /
// Use this function when joining paths to force the returned path to use / as the path separator
// This will improve cross-platform compatibility
func JoinPath(elem ...string) string {
	return filepath.ToSlash(filepath.Join(elem...))
}

// Use this function when cleaning paths to ensure the returned path uses / as the path separator to improve cross-platform compatibility
func CleanPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// ExpandArguments expands the list of arguments like x shell do
func ExpandArguments(args []string, folder string) (result []string) {
	prefix := folder + string(filepath.Separator)

	for _, arg := range args {
		if strings.ContainsAny(arg, "*?[]") {
			if !filepath.IsAbs(arg) {
				arg = prefix + arg
			}
			if expanded, _ := filepath.Glob(arg); expanded != nil {
				for i := range expanded {
					// We remove the prefix from the result as if it was executed directly in the folder directory
					expanded[i] = strings.TrimPrefix(expanded[i], prefix)
				}
				result = append(result, expanded...)
				continue
			}
		}
		result = append(result, arg)
	}
	return
}
