package util

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/multilogger"
	"github.com/coveooss/terragrunt/v2/awshelper"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	getter "github.com/hashicorp/go-getter/v2"
	"github.com/sirupsen/logrus"
	"gopkg.in/matryer/try.v1"
)

// FileStat calls os.Stat with retries
// When multiple projects are running in parallel, the os.Stat() function
// returns 'bad file descriptor' if the file is being overwritten while being read.
func FileStat(path string) (result os.FileInfo, err error) {
	for retries := 0; ; {
		if result, err = os.Stat(path); err != nil && strings.Contains(fmt.Sprint(err), "bad file descriptor") {
			if retries < 5 {
				time.Sleep(10 * time.Millisecond)
				retries++
				continue
			}
		}
		return
	}
}

// FileExists returns true if the given file exists.
func FileExists(path string) bool {
	_, err := FileStat(path)
	return err == nil
}

// CanonicalPath returns the canonical version of the given path, relative to the given base path. That is, if the given path is a
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

// CanonicalPaths returns the canonical version of the given paths, relative to the given base path. That is, if a given path
// is a relative path, assume it is relative to the given base path. A canonical path is an absolute path with all relative
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

// DeleteFiles deletes the given list of files. Note: this function ONLY deletes files and will return an error if you
// pass in a folder path.
func DeleteFiles(files []string) error {
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return tgerrors.WithStackTrace(err)
		}
	}
	return nil
}

// Grep returns true if the given regex can be found in any of the files matched by the given glob
func Grep(regex *regexp.Regexp, folder string, patterns ...string) (bool, error) {
	matches, err := utils.FindFiles(folder, false, false, patterns...)
	if err != nil {
		return false, tgerrors.WithStackTrace(err)
	}

	for _, match := range matches {
		bytes, err := ioutil.ReadFile(match)
		if err != nil {
			return false, tgerrors.WithStackTrace(err)
		}

		if regex.Match(bytes) {
			return true, nil
		}
	}

	return false, nil
}

// GetPathRelativeTo returns the relative path you would have to take to get from basePath to path
func GetPathRelativeTo(path, basePath string) (string, error) {
	if path == "" {
		path = "."
	}
	if basePath == "" {
		basePath = "."
	}

	inputFolderAbs, err := filepath.Abs(basePath)
	if err != nil {
		return "", tgerrors.WithStackTrace(err)
	}

	fileAbs, err := filepath.Abs(path)
	if err != nil {
		return "", tgerrors.WithStackTrace(err)
	}

	relPath, err := filepath.Rel(inputFolderAbs, fileAbs)
	if err != nil {
		return "", tgerrors.WithStackTrace(err)
	}

	return filepath.ToSlash(relPath), nil
}

// GetPathRelativeToWorkingDir returns the path relative to the current working directory
func GetPathRelativeToWorkingDir(path string) (result string) {
	currentDir, err := os.Getwd()
	result = path
	if err == nil {
		result, _ = GetPathRelativeTo(path, currentDir)
	}
	return
}

// GetPathRelativeToWorkingDirMax returns either an absolute path or a relative path if it is not too far
// from relatively to the current directory
func GetPathRelativeToWorkingDirMax(path string, maxLevel uint) (result string) {
	result = GetPathRelativeToWorkingDir(path)

	sep := strings.Repeat(".."+string(os.PathSeparator), int(maxLevel+1))
	if filepath.IsAbs(path) && strings.HasPrefix(result, sep) {
		// If the path is absolute and it is too far from the current folder
		result = path
	}
	return
}

// GetPathRelativeToMax returns either an absolute path or a relative path if it is not too far
// from relatively to the base path
func GetPathRelativeToMax(path, basePath string, maxLevel uint) (result string) {
	result, err := GetPathRelativeTo(path, basePath)
	if err != nil {
		result = path
	} else {
		sep := strings.Repeat(".."+string(os.PathSeparator), int(maxLevel+1))
		if filepath.IsAbs(path) && strings.HasPrefix(result, sep) {
			// If the path is absolute and it is too far from the current folder
			result = path
		}
	}
	return
}

// ReadFileAsString returns the contents of the file at the given path as a string
func ReadFileAsString(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", tgerrors.WithStackTraceAndPrefix(err, "Error reading file at path %s", path)
	}

	return string(bytes), nil
}

// ReadFileAsStringFromSource returns the contents of the file at the given path
// from an external source (github, s3, etc.) as a string
// It uses terraform to execute its command
func ReadFileAsStringFromSource(source, path string, logger *multilogger.Logger) (localFile, content string, err error) {
	cacheDir, err := GetSource(source, "", logger, "")
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
// if the content has not changed. When given a local path, it is directly returned
func GetSource(source, pwd string, logger *multilogger.Logger, fileRegex string) (string, error) {
	logf := func(level logrus.Level, format string, args ...interface{}) {
		if logger != nil {
			logger.Logf(level, format, args...)
		}
	}

	var result string
	if finalErr := try.Do(func(attempt int) (bool, error) {
		var err error
		result, err = getSource(source, pwd, logf, fileRegex)
		if err != nil {
			if attempt > 3 || errors.Is(err, awshelper.ErrS3PathNotFoundError) {
				// If the object doesn't exist in S3, there's no point in retrying
				// false tells Try.Do to not retry
				return false, err
			}
			logf(logrus.WarnLevel, "Downloading %s failed. Retrying in 1 second. Err: %v", source, err)
			time.Sleep(time.Second)
			delete(sharedContent, result)
			if result != "" && FileExists(result) {
				// Download failed but the dir exists, let's delete it
				logf(logrus.WarnLevel, "Deleting cache dir for %s: %s", source, result)
				if removeErr := os.RemoveAll(result); removeErr != nil {
					logf(logrus.WarnLevel, "Failed to delete cache dir %s: %v", result, removeErr)
				}
			}

		}
		// Try.Do will retry if err is not nil
		return true, err
	}); finalErr != nil {
		return "", fmt.Errorf("%w while copying source from %s", finalErr, source)
	}
	return result, nil
}

func getSource(source, pwd string, logf func(level logrus.Level, format string, args ...interface{}), fileRegex string) (string, error) {
	logf(logrus.TraceLevel, "Converting S3 path to be compatible with getter for %s", source)
	source, err := awshelper.ConvertS3Path(source)
	if err != nil {
		return "", err
	}

	logf(logrus.TraceLevel, "Calling getter.Detect (pattern matching in the getter library) on %s", source)
	source, err = getter.Detect(source, pwd, getter.Detectors)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(source, "file://") {
		logf(logrus.TraceLevel, "Getting files directly (without copy) from %s", source)
		return strings.Replace(source, "file://", "", 1), nil
	}

	cacheKey := fmt.Sprintf("%s (Regex: %s)", source, fileRegex)
	logf(logrus.TraceLevel, "Fetching and locking cache for %s", cacheKey)
	cacheDir := GetTempDownloadFolder("terragrunt-cache", EncodeBase64Sha1(cacheKey))
	sharedMutex.Lock()
	defer sharedMutex.Unlock()

	logf(logrus.TraceLevel, "Getting S3 bucket info for %s", source)
	s3Object, _ := awshelper.GetBucketObjectInfoFromURL(source)
	if s3Object != nil {
		logf(logrus.TraceLevel, "Confirmed that this is an S3 object. Checking status for %s", source)
		source = s3Object.String()
		err = awshelper.CheckS3Status(s3Object, cacheDir)
		if errors.Is(err, awshelper.ErrS3PathNotFoundError) {
			logf(logrus.DebugLevel, "%s was not found in S3", source)
			// If the source is not found in S3, return right away
			return "", err
		}
	}

	_, alreadyInCache := sharedContent[cacheDir]
	if !alreadyInCache || err != nil {
		logf(logrus.DebugLevel, "Adding %s to the in-memory cache", source)
		if !FileExists(cacheDir) || err != nil {
			var reason string
			if err != nil {
				reason = fmt.Sprintf("because status = %v", err)
			}
			logf(logrus.DebugLevel, "Getting source files %s %s", cacheKey, reason)
			if !alreadyInCache {
				err = os.RemoveAll(cacheDir)
				if err != nil {
					return "", fmt.Errorf("%w while deleting the cache for %s", err, source)
				}
			}

			if err := GetCopy(cacheDir, source, fileRegex); err != nil {
				return "", fmt.Errorf("caught error while fetching files from %s: %w", source, err)
			}

			if s3Object != nil {
				// Since it is an S3 Bucket object, we save its md5 value
				// to avoid multiple download of the same object
				err = awshelper.SaveS3Status(s3Object, cacheDir)
			}
			if err != nil {
				return "", fmt.Errorf("caught %w while saving status for %s", err, source)
			}
			logf(logrus.DebugLevel, "Files from %s successfully added to the cache at %s", source, cacheDir)
		}
		sharedContent[cacheDir] = true
	} else {
		logf(logrus.TraceLevel, "Already in cache: %s", source)
	}

	return cacheDir, nil
}

var sharedMutex sync.Mutex
var sharedContent = map[string]bool{}

// CopyFolderContents copies the files and folders within the source folder into the destination folder. Note that hidden files and folders
// (those starting with a dot) will be skipped.
func CopyFolderContents(source, destination string, excluded ...string) error {
	files, err := ioutil.ReadDir(source)
	if err != nil {
		return tgerrors.WithStackTrace(err)
	}

	for _, file := range files {
		src := filepath.Join(source, file.Name())
		dest := filepath.Join(destination, file.Name())
		if ListContainsElement(excluded, src) {
			continue
		}
		if PathContainsHiddenFileOrFolder(src) {
			continue
		} else if file.IsDir() {
			if err := os.MkdirAll(dest, file.Mode()); err != nil {
				return tgerrors.WithStackTrace(err)
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

// PathContainsHiddenFileOrFolder returns true if folder contains files starting with . (except . and ..)
func PathContainsHiddenFileOrFolder(path string) bool {
	pathParts := strings.Split(path, string(filepath.Separator))
	for _, pathPart := range pathParts {
		if strings.HasPrefix(pathPart, ".") && pathPart != "." && pathPart != ".." {
			return true
		}
	}
	return false
}

// CopyFile copies a file from source to destination
func CopyFile(source string, destination string) error {
	contents, err := ioutil.ReadFile(source)
	if err != nil {
		return tgerrors.WithStackTrace(err)
	}

	return WriteFileWithSamePermissions(source, destination, contents)
}

// WriteFileWithSamePermissions writes a file to the given destination with the given contents using the same permissions as the file at source
func WriteFileWithSamePermissions(source string, destination string, contents []byte) error {
	fileInfo, err := FileStat(source)

	if err != nil {
		return tgerrors.WithStackTrace(err)
	}

	return ioutil.WriteFile(destination, contents, fileInfo.Mode())
}

// JoinPath always use / as the separator.
// Windows systems use \ as the path separator *nix uses /
// Use this function when joining paths to force the returned path to use / as the path separator
// This will improve cross-platform compatibility
func JoinPath(elem ...string) string {
	return filepath.ToSlash(filepath.Join(elem...))
}

// CleanPath is used to clean paths to ensure the returned path uses / as the path separator to improve cross-platform compatibility.
func CleanPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// ExpandArguments expands the list of arguments like x shell do
func ExpandArguments(args []interface{}, folder string) (result []interface{}) {
	prefix := folder + string(filepath.Separator)

	for _, argI := range args {
		// We consider \$ as an escape char for $ and we do not want ExpandEnv to replace it right now
		const stringEscape = "%StrEsc%"
		arg := fmt.Sprint(argI)
		arg = strings.Replace(arg, `\$`, stringEscape, -1)
		arg = strings.Replace(os.ExpandEnv(arg), stringEscape, "$", -1)
		if strings.ContainsAny(arg, "*?[]") && !strings.ContainsAny(arg, "$|`") && !strings.HasPrefix(arg, "-") {
			// The string contains wildcard and is not a shell command
			originalArg := arg
			if !filepath.IsAbs(arg) {
				arg = prefix + arg
			}
			expanded, _ := filepath.Glob(arg)
			if len(expanded) > 0 {
				for i := range expanded {
					// We remove the prefix from the result as if it was executed directly in the folder directory
					expanded[i] = strings.TrimPrefix(expanded[i], prefix)
					result = append(result, expanded[i])
				}
			} else {
				// there was nothing to expand, reappend the original arg
				result = append(result, originalArg)
			}
			continue
		}
		result = append(result, arg)
	}
	return
}
