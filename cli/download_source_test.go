package cli

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/stretchr/testify/assert"
)

func TestAlreadyHaveLatestCodeLocalFilePath(t *testing.T) {
	t.Parallel()

	canonicalURL := fmt.Sprintf("file://%s", absPath(t, "../test/fixture-download-source/hello-world"))
	downloadDir := "does-not-exist"

	testAlreadyHaveLatestCode(t, canonicalURL, downloadDir, false)
}

func TestAlreadyHaveLatestCodeRemoteFilePathDownloadDirDoesNotExist(t *testing.T) {
	t.Parallel()

	canonicalURL := "http://www.some-url.com"
	downloadDir := "does-not-exist"

	testAlreadyHaveLatestCode(t, canonicalURL, downloadDir, false)
}

func TestAlreadyHaveLatestCodeRemoteFilePathDownloadDirExistsNoVersionNoVersionFile(t *testing.T) {
	t.Parallel()

	canonicalURL := "http://www.some-url.com"
	downloadDir := "../test/fixture-download-source/download-dir-empty"

	testAlreadyHaveLatestCode(t, canonicalURL, downloadDir, false)
}

func TestAlreadyHaveLatestCodeRemoteFilePathDownloadDirExistsNoVersionWithVersionFile(t *testing.T) {
	t.Parallel()

	canonicalURL := "http://www.some-url.com"
	downloadDir := "../test/fixture-download-source/download-dir-version-file-no-query"

	testAlreadyHaveLatestCode(t, canonicalURL, downloadDir, true)
}

func TestAlreadyHaveLatestCodeRemoteFilePathDownloadDirExistsWithVersionNoVersionFile(t *testing.T) {
	t.Parallel()

	canonicalURL := "http://www.some-url.com?ref=v0.0.1"
	downloadDir := "../test/fixture-download-source/download-dir-empty"

	testAlreadyHaveLatestCode(t, canonicalURL, downloadDir, false)
}

func TestAlreadyHaveLatestCodeRemoteFilePathDownloadDirExistsWithVersionAndVersionFile(t *testing.T) {
	t.Parallel()

	canonicalURL := "http://www.some-url.com?ref=v0.0.1"
	downloadDir := "../test/fixture-download-source/download-dir-version-file"

	testAlreadyHaveLatestCode(t, canonicalURL, downloadDir, true)
}

func TestDownloadTerraformSourceIfNecessaryLocalDirToEmptyDir(t *testing.T) {
	t.Parallel()

	canonicalURL := fmt.Sprintf("file://%s", absPath(t, "../test/fixture-download-source/hello-world"))
	downloadDir := tmpDir(t)
	defer os.Remove(downloadDir)

	testDownloadTerraformSourceIfNecessary(t, canonicalURL, downloadDir, false, "# Hello, World")
}

func TestDownloadTerraformSourceIfNecessaryLocalDirToAlreadyDownloadedDir(t *testing.T) {
	t.Parallel()

	canonicalURL := fmt.Sprintf("file://%s", absPath(t, "../test/fixture-download-source/hello-world"))
	downloadDir := tmpDir(t)
	defer os.Remove(downloadDir)

	copyFolder(t, "../test/fixture-download-source/hello-world-2", downloadDir)

	testDownloadTerraformSourceIfNecessary(t, canonicalURL, downloadDir, false, "# Hello, World")
}

func TestDownloadTerraformSourceIfNecessaryRemoteUrlToEmptyDir(t *testing.T) {
	t.Parallel()

	canonicalURL := "github.com/coveooss/terragrunt//test/fixture-download-source/hello-world"
	downloadDir := tmpDir(t)
	defer os.Remove(downloadDir)

	testDownloadTerraformSourceIfNecessary(t, canonicalURL, downloadDir, false, "# Hello, World")
}

func TestDownloadTerraformSourceIfNecessaryRemoteUrlToAlreadyDownloadedDir(t *testing.T) {
	t.Parallel()

	canonicalURL := "github.com/coveooss/terragrunt//test/fixture-download-source/hello-world"
	downloadDir := tmpDir(t)
	defer os.Remove(downloadDir)

	copyFolder(t, "../test/fixture-download-source/hello-world-2", downloadDir)

	testDownloadTerraformSourceIfNecessary(t, canonicalURL, downloadDir, false, "# Hello, World 2")
}

func TestDownloadTerraformSourceIfNecessaryRemoteUrlToAlreadyDownloadedDirDifferentVersion(t *testing.T) {
	t.Parallel()

	canonicalURL := "github.com/coveooss/terragrunt//test/fixture-download-source/hello-world?ref=v1.0.0"
	downloadDir := tmpDir(t)
	defer os.Remove(downloadDir)

	copyFolder(t, "../test/fixture-download-source/hello-world-2", downloadDir)

	testDownloadTerraformSourceIfNecessary(t, canonicalURL, downloadDir, false, "# Hello, World")
}

func TestDownloadTerraformSourceIfNecessaryRemoteUrlToAlreadyDownloadedDirSameVersion(t *testing.T) {
	t.Parallel()

	// The version specified does not have to exist, we just want to compare the hash computed from the url with the content of the file
	canonicalURL := "github.com/coveooss/terragrunt//test/fixture-download-source/hello-world?ref=v0.9.7"
	downloadDir := tmpDir(t)
	defer os.Remove(downloadDir)

	copyFolder(t, "../test/fixture-download-source/hello-world-version-remote", downloadDir)

	testDownloadTerraformSourceIfNecessary(t, canonicalURL, downloadDir, false, "# Hello, World version remote")
}

func TestDownloadTerraformSourceIfNecessaryRemoteUrlOverrideSource(t *testing.T) {
	t.Parallel()

	canonicalURL := "github.com/coveooss/terragrunt//test/fixture-download-source/hello-world?ref=v1.0.0"
	downloadDir := tmpDir(t)
	defer os.Remove(downloadDir)

	copyFolder(t, "../test/fixture-download-source/hello-world-version-remote", downloadDir)

	testDownloadTerraformSourceIfNecessary(t, canonicalURL, downloadDir, true, "# Hello, World")
}

func testDownloadTerraformSourceIfNecessary(t *testing.T, canonicalURL string, downloadDir string, sourceUpdate bool, expectedFileContents string) {
	terraformSource := &TerraformSource{
		CanonicalSourceURL: parseURL(t, canonicalURL),
		DownloadDir:        downloadDir,
		WorkingDir:         downloadDir,
		VersionFile:        util.JoinPath(downloadDir, "version-file.txt"),
	}

	terragruntOptions := options.NewTerragruntOptionsForTest("./should-not-be-used")
	terragruntOptions.SourceUpdate = sourceUpdate

	err := downloadTerraformSourceIfNecessary(terraformSource, terragruntOptions)
	assert.Nil(t, err, "For terraform source %v: %v", terraformSource, err)

	expectedFilePath := util.JoinPath(downloadDir, "main.tf")
	if assert.True(t, util.FileExists(expectedFilePath), "For terraform source %v", terraformSource) {
		actualFileContents := readFile(t, expectedFilePath)
		assert.Equal(t, expectedFileContents, actualFileContents, "For terraform source %v", terraformSource)
	}
}

func testAlreadyHaveLatestCode(t *testing.T, canonicalURL string, downloadDir string, expected bool) {
	terraformSource := &TerraformSource{
		CanonicalSourceURL: parseURL(t, canonicalURL),
		DownloadDir:        downloadDir,
		WorkingDir:         downloadDir,
		VersionFile:        util.JoinPath(downloadDir, "version-file.txt"),
	}

	actual, err := alreadyHaveLatestCode(terraformSource)
	assert.Nil(t, err, "Unexpected error for terraform source %v: %v", terraformSource, err)
	assert.Equal(t, expected, actual, "For terraform source %v", terraformSource)
}

func tmpDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "download-source-test")
	if err != nil {
		t.Fatal(err)
	}
	return filepath.FromSlash(dir)
}

func absPath(t *testing.T, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func parseURL(t *testing.T, str string) *url.URL {
	// URLs should have only forward slashes, whereas on Windows, the file paths may be backslashes
	rawURL := strings.Join(strings.Split(str, string(filepath.Separator)), "/")

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func readFile(t *testing.T, path string) string {
	contents, err := util.ReadFileAsString(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func copyFolder(t *testing.T, src string, dest string) {
	err := util.CopyFolderContents(filepath.FromSlash(src), filepath.FromSlash(dest))
	if err != nil {
		t.Fatal(err)
	}
}
