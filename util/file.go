package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/hashicorp/terraform/config/module"
	logging "github.com/op/go-logging"
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

// Return the path relative to the current working directory
func GetPathRelativeToWorkingDir(path string) (result string) {
	currentDir, err := os.Getwd()
	result = path
	if err == nil {
		result, err = GetPathRelativeTo(path, currentDir)
	}
	return
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
func ReadFileAsStringFromSource(source, path string, logger *logging.Logger) (localFile, content string, err error) {
	cacheDir, err := GetSource(source, logger)
	if err != nil {
		return "", "", err
	}

	localFile = filepath.Join(cacheDir, path)
	content, err = ReadFileAsString(localFile)
	return
}

// GetTempDownloadFolder returns the fo
func GetTempDownloadFolder(folders ...string) string {
	tempFolder := os.Getenv("TERRAGRUNT_CACHE")
	if tempFolder == "" {
		tempFolder = os.TempDir()
	}
	return filepath.Join(append([]string{tempFolder}, folders...)...)
}

// GetSource gets the content of the source in a temporary folder and returns
// the local path. The function manages a cache to avoid multiple remote calls
// if the content has not changed
func GetSource(source string, logger *logging.Logger) (string, error) {
	path, err := aws_helper.ConvertS3Path(source)
	if err != nil {
		return "", err
	}

	cacheDir := GetTempDownloadFolder("terragrunt-cache", EncodeBase64Sha1(source))
	sharedMutex.Lock()
	defer sharedMutex.Unlock()

	upToDate, err := aws_helper.CheckS3Status(cacheDir)
	if _, ok := sharedContent[cacheDir]; !ok || !upToDate || err != nil {
		if logger != nil {
			logger.Infof("Adding %s to the cache, expired=%v", source, !upToDate)
		}
		if !FileExists(cacheDir) || !upToDate {
			if logger != nil {
				logger.Info("Getting source files", source, "from", path)
			}
			os.RemoveAll(cacheDir)

			err = module.GetCopy(cacheDir, path)
			if err != nil {
				return "", fmt.Errorf("%v while copying source from %s", err, path)
			}

			err = aws_helper.SaveS3Status(source, cacheDir)
			if err != nil {
				return "", fmt.Errorf("%v while saving status for %s", err, path)
			}
			if logger != nil {
				logger.Info("Files from", source, "successfully added to the cache at", cacheDir)
			}
		}
		sharedContent[cacheDir] = true
	}
	return cacheDir, nil
}

var sharedMutex sync.Mutex
var sharedContent = map[string]bool{}

// Copy the files and folders within the source folder into the destination folder. Note that hidden files and folders
// (those starting with a dot) will be skipped.
func CopyFolderContents(source string, destination string) error {
	files, err := ioutil.ReadDir(source)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	for _, file := range files {
		src := filepath.Join(source, file.Name())
		dest := filepath.Join(destination, file.Name())

		if PathContainsHiddenFileOrFolder(src) {
			continue
		} else if file.IsDir() {
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

func PathContainsHiddenFileOrFolder(path string) bool {
	pathParts := strings.Split(path, string(filepath.Separator))
	for _, pathPart := range pathParts {
		if strings.HasPrefix(pathPart, ".") && pathPart != "." && pathPart != ".." {
			return true
		}
	}
	return false
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
		arg = os.ExpandEnv(arg)
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

// FindFiles returns the list of files in the specified folder that match one of the supplied patterns
func FindFiles(folder string, patterns ...string) ([]string, error) {
	var tfFiles []string
	for _, ext := range patterns {
		files, err := filepath.Glob(filepath.Join(folder, ext))
		if err != nil {
			return nil, err
		}
		tfFiles = append(tfFiles, files...)
	}
	return tfFiles, nil
}
