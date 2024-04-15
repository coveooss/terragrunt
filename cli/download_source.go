package cli

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/hashicorp/go-getter"
	urlhelper "github.com/hashicorp/go-getter/helper/url"
)

// TerraformSource represents information about Terraform source code that needs to be downloaded
type TerraformSource struct {
	// A canonical version of RawSource, in URL format
	CanonicalSourceURL *url.URL

	// The folder where we should download the source to
	DownloadDir string

	// The folder in DownloadDir that should be used as the working directory for Terraform
	WorkingDir string

	// The path to a file in DownloadDir that stores the version number of the code
	VersionFile string
}

func (src *TerraformSource) String() string {
	return fmt.Sprintf("TerraformSource{CanonicalSourceURL = %v, DownloadDir = %v, WorkingDir = %v, VersionFile = %v}", src.CanonicalSourceURL, src.DownloadDir, src.WorkingDir, src.VersionFile)
}

var forcedRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

// 1. Download the given source URL, which should use Terraform's module source syntax, into a temporary folder
// 2. Copy the contents of terragruntOptions.WorkingDir into the temporary folder.
// 3. Set terragruntOptions.WorkingDir to the temporary folder.
//
// See the processTerraformSource method for how we determine the temporary folder so we can reuse it across multiple
// runs of Terragrunt to avoid downloading everything from scratch every time.
func downloadTerraformSource(source *TerraformSource, terragruntOptions *options.TerragruntOptions) error {
	if err := downloadTerraformSourceIfNecessary(source, terragruntOptions); err != nil {
		return err
	}

	terragruntOptions.Logger.Debugf("Copying files from %s into %s", terragruntOptions.WorkingDir, source.WorkingDir)
	excluded := []string{}
	if source.CanonicalSourceURL.Scheme == "file" {
		// We exclude the local source path from the copy to avoid copying source directory twice in the temporary folder.
		excluded = append(excluded, source.CanonicalSourceURL.Path)
	}
	if err := util.CopyFolderContents(terragruntOptions.WorkingDir, source.WorkingDir, excluded...); err != nil {
		return err
	}

	terragruntOptions.Logger.Debug("Setting working directory to", source.WorkingDir)
	terragruntOptions.WorkingDir = source.WorkingDir

	return nil
}

// Download the specified TerraformSource if the latest code hasn't already been downloaded.
func downloadTerraformSourceIfNecessary(terraformSource *TerraformSource, terragruntOptions *options.TerragruntOptions) error {
	if terragruntOptions.SourceUpdate {
		terragruntOptions.Logger.Debugf("The --%s flag is set, so deleting the temporary folder %s before downloading source.", optTerragruntSourceUpdate, terraformSource.DownloadDir)
		if err := os.RemoveAll(terraformSource.DownloadDir); err != nil {
			return tgerrors.WithStackTrace(err)
		}
	}

	alreadyLatest, err := alreadyHaveLatestCode(terraformSource)
	if err != nil {
		return err
	}

	if alreadyLatest {
		terragruntOptions.Logger.Debugf("Terraform files in %s are up to date. Will not download again.", terraformSource.WorkingDir)
		return nil
	}

	if err := cleanupTerraformFiles(terraformSource.DownloadDir, terragruntOptions); err != nil {
		return err
	}

	terragruntOptions.Logger.Debugf("Downloading Terraform configurations from %s into %s", terraformSource.CanonicalSourceURL, terraformSource.DownloadDir)
	if err := util.GetCopy(terraformSource.DownloadDir, terraformSource.CanonicalSourceURL.String(), ""); err != nil {
		return err
	}

	return writeVersionFile(terraformSource)
}

// Returns true if the specified TerraformSource, of the exact same version, has already been downloaded into the
// DownloadFolder. This helps avoid downloading the same code multiple times. Note that if the TerraformSource points
// to a local file path, we assume the user is doing local development and always return false to ensure the latest
// code is downloaded (or rather, copied) every single time. See the processTerraformSource method for more info.
func alreadyHaveLatestCode(terraformSource *TerraformSource) (bool, error) {
	if isLocalSource(terraformSource.CanonicalSourceURL) ||
		!util.FileExists(terraformSource.DownloadDir) ||
		!util.FileExists(terraformSource.WorkingDir) ||
		!util.FileExists(terraformSource.VersionFile) {

		return false, nil
	}

	currentVersion := encodeSourceVersion(terraformSource.CanonicalSourceURL)
	previousVersion, err := readVersionFile(terraformSource)

	if err != nil {
		return false, err
	}

	return previousVersion == currentVersion, nil
}

// Return the version number stored in the DownloadDir. This version number can be used to check if the Terraform code
// that has already been downloaded is the same as the version the user is currently requesting. The version number is
// calculated using the encodeSourceVersion method.
func readVersionFile(terraformSource *TerraformSource) (string, error) {
	return util.ReadFileAsString(terraformSource.VersionFile)
}

// Write a file into the DownloadDir that contains the version number of this source code. The version number is
// calculated using the encodeSourceVersion method.
func writeVersionFile(terraformSource *TerraformSource) error {
	version := encodeSourceVersion(terraformSource.CanonicalSourceURL)
	return tgerrors.WithStackTrace(ioutil.WriteFile(terraformSource.VersionFile, []byte(version), 0640))
}

// Take the given source path and create a TerraformSource struct from it, including the folder where the source should
// be downloaded to. Our goal is to reuse the download folder for the same source URL between Terragrunt runs.
// Otherwise, for every Terragrunt command, you'd have to wait for Terragrunt to download your Terraform code, download
// that code's dependencies (terraform get), and configure remote state (terraform remote config), which is very slow.
//
// To maximize reuse, given a working directory w and a source URL s, we download code from S into the folder /T/W/H
// where:
//
// 1. S is the part of s before the double-slash (//). This typically represents the root of the repo (e.g.
//    github.com/foo/infrastructure-modules). We download the entire repo so that relative paths to other files in that
//    repo resolve correctly. If no double-slash is specified, all of s is used.
// 1. T is the OS temp dir (e.g. /tmp).
// 2. W is the base 64 encoded sha1 hash of w. This ensures that if you are running Terragrunt concurrently in
//    multiple folders (e.g. during automated tests), then even if those folders are using the same source URL s, they
//    do not overwrite each other.
// 3. H is the base 64 encoded sha1 of S without its query string. For remote source URLs (e.g. Git
//    URLs), this is based on the assumption that the scheme/host/path of the URL (e.g. git::github.com/foo/bar)
//    identifies the repo, and we always want to download the same repo into the same folder (see the encodeSourceName
//    method). We also assume the version of the module is stored in the query string (e.g. ref=v0.0.3), so we store
//    the base 64 encoded sha1 of the query string in a file called .terragrunt-source-version within /T/W/H.
//
// The downloadTerraformSourceIfNecessary decides when we should download the Terraform code and when not to. It uses
// the following rules:
//
// 1. Always download source URLs pointing to local file paths.
// 2. Only download source URLs pointing to remote paths if /T/W/H doesn't already exist or, if it does exist, if the
//    version number in /T/W/H/.terragrunt-source-version doesn't match the current version.
func processTerraformSource(source string, terragruntOptions *options.TerragruntOptions) (*TerraformSource, error) {
	canonicalWorkingDir, err := util.CanonicalPath(terragruntOptions.WorkingDir, "")
	if err != nil {
		return nil, err
	}

	canonicalSourceURL, err := toSourceURL(source, canonicalWorkingDir)
	if err != nil {
		return nil, err
	}

	rootSourceURL, modulePath, err := splitSourceURL(canonicalSourceURL, terragruntOptions)
	if err != nil {
		return nil, err
	}

	if isLocalSource(rootSourceURL) {
		// Always use canonical file paths for local source folders, rather than relative paths, to ensure
		// that the same local folder always maps to the same download folder, no matter how the local folder
		// path is specified
		canonicalFilePath, err := util.CanonicalPath(rootSourceURL.Path, "")
		if err != nil {
			return nil, err
		}
		rootSourceURL.Path = canonicalFilePath
	}

	rootPath, err := encodeSourceName(rootSourceURL)
	if err != nil {
		return nil, err
	}

	// We add the uniqueness factor to the folder name to ensure that distinct environment are processed in
	// distinct directory
	encodedWorkingDir := util.EncodeBase64Sha1(canonicalWorkingDir + terragruntOptions.UniquenessCriteria)
	downloadDir := util.JoinPath(terragruntOptions.DownloadDir, encodedWorkingDir, rootPath)
	workingDir := util.JoinPath(downloadDir, modulePath)
	versionFile := util.JoinPath(downloadDir, ".terragrunt-source-version")

	return &TerraformSource{
		CanonicalSourceURL: rootSourceURL,
		DownloadDir:        downloadDir,
		WorkingDir:         workingDir,
		VersionFile:        versionFile,
	}, nil
}

// Convert the given source into a URL struct. This method should be able to handle all source URLs that the terraform
// init command can handle, parsing local file paths, Git paths, and HTTP URLs correctly.
func toSourceURL(source string, workingDir string) (*url.URL, error) {
	// The go-getter library is what Terraform's init command uses to download source URLs. Use that library to
	// parse the URL.
	rawSourceURLWithGetter, err := getter.Detect(source, workingDir, getter.Detectors)
	if err != nil {
		return nil, tgerrors.WithStackTrace(err)
	}

	return parseSourceURL(rawSourceURLWithGetter)
}

// Parse the given source URL into a URL struct. This method can handle source URLs that include go-getter's "forced
// getter" prefixes, such as git::.
func parseSourceURL(source string) (*url.URL, error) {
	forcedGetter, rawSourceURL := getForcedGetter(source)

	// Parse the URL without the getter prefix
	canonicalSourceURL, err := urlhelper.Parse(rawSourceURL)
	if err != nil {
		return nil, tgerrors.WithStackTrace(err)
	}

	// Reattach the "getter" prefix as part of the scheme
	if forcedGetter != "" {
		canonicalSourceURL.Scheme = fmt.Sprintf("%s::%s", forcedGetter, canonicalSourceURL.Scheme)
	}

	return canonicalSourceURL, nil
}

// Terraform source URLs can contain a "getter" prefix that specifies the type of protocol to use to download that URL,
// such as "git::", which means Git should be used to download the URL. This method returns the getter prefix and the
// rest of the URL. This code is copied from the getForcedGetter method of go-getter/get.go, as that method is not
// exported publicly.
func getForcedGetter(sourceURL string) (string, string) {
	if matches := forcedRegexp.FindStringSubmatch(sourceURL); len(matches) > 2 {
		return matches[1], matches[2]
	}

	return "", sourceURL
}

// Splits a source URL into the root repo and the path. The root repo is the part of the URL before the double-slash
// (//), which typically represents the root of a modules repo (e.g. github.com/foo/infrastructure-modules) and the
// path is everything after the double slash. If there is no double-slash in the URL, the root repo is the entire
// sourceURL and the path is an empty string.
func splitSourceURL(sourceURL *url.URL, terragruntOptions *options.TerragruntOptions) (*url.URL, string, error) {
	pathSplitOnDoubleSlash := strings.SplitN(sourceURL.Path, "//", 2)

	if len(pathSplitOnDoubleSlash) > 1 {
		sourceURLModifiedPath, err := parseSourceURL(sourceURL.String())
		if err != nil {
			return nil, "", tgerrors.WithStackTrace(err)
		}

		sourceURLModifiedPath.Path = pathSplitOnDoubleSlash[0]
		return sourceURLModifiedPath, pathSplitOnDoubleSlash[1], nil
	}
	terragruntOptions.Logger.Debugf("No double-slash (//) found in source URL %s. Relative paths in downloaded Terraform code may not work.", sourceURL.Path)
	return sourceURL, "", nil
}

// Encode a version number for the given source URL. When calculating a version number, we simply take the query
// string of the source URL, calculate its sha1, and base 64 encode it. For remote URLs (e.g. Git URLs), this is
// based on the assumption that the scheme/host/path of the URL (e.g. git::github.com/foo/bar) identifies the module
// name and the query string (e.g. ?ref=v0.0.3) identifies the version. For local file paths, there is no query string,
// so the same file path (/foo/bar) is always considered the same version. See also the encodeSourceName and
// processTerraformSource methods.
func encodeSourceVersion(sourceURL *url.URL) string {
	return util.EncodeBase64Sha1(sourceURL.Query().Encode())
}

// Encode a the module name for the given source URL. When calculating a module name, we calculate the base 64 encoded
// sha1 of the entire source URL without the query string. For remote URLs (e.g. Git URLs), this is based on the
// assumption that the scheme/host/path of the URL (e.g. git::github.com/foo/bar) identifies the module name and the
// query string (e.g. ?ref=v0.0.3) identifies the version. For local file paths, there is no query string, so the same
// file path (/foo/bar) is always considered the same version. See also the encodeSourceVersion and
// processTerraformSource methods.
func encodeSourceName(sourceURL *url.URL) (string, error) {
	sourceURLNoQuery, err := parseSourceURL(sourceURL.String())
	if err != nil {
		return "", tgerrors.WithStackTrace(err)
	}

	sourceURLNoQuery.RawQuery = ""

	return util.EncodeBase64Sha1(sourceURLNoQuery.String()), nil
}

// Returns true if the given URL refers to a path on the local file system
func isLocalSource(sourceURL *url.URL) bool {
	return sourceURL.Scheme == "file"
}

// If this temp folder already exists, simply delete all the Terraform files within it
// (the terraform init command will redownload the latest ones), but leave all the other files, such
// as the .terraform folder with the downloaded modules and remote state settings.
func cleanupTerraformFiles(path string, terragruntOptions *options.TerragruntOptions) error {
	if !util.FileExists(path) {
		return nil
	}

	terragruntOptions.Logger.Debug("Cleaning up existing terraform files in", path)

	files, err := utils.FindFiles(path, true, false, options.TerraformFilesTemplates...)
	if err != nil {
		return tgerrors.WithStackTrace(err)
	}

	// Filter out files in .terraform folders, since those are from modules downloaded via a call to terraform get,
	// and we don't want to re-download them.
	filteredFiles := []string{}
	for _, file := range files {
		if !strings.Contains(file, ".terraform") {
			filteredFiles = append(filteredFiles, file)
		}
	}

	return util.DeleteFiles(filteredFiles)
}

// There are two ways a user can tell Terragrunt that it needs to download Terraform configurations from a specific
// URL: via a command-line option or via an entry in the Terragrunt configuration. If the user used one of these, this
// method returns the source URL and the boolean true; if not, this method returns an empty string and false.
func getTerraformSourceURL(terragruntOptions *options.TerragruntOptions, terragruntConfig *config.TerragruntConfig) (string, bool) {
	if terragruntOptions.Source != "" {
		return terragruntOptions.Source, true
	} else if terragruntConfig.Terraform != nil && terragruntConfig.Terraform.Source != "" {
		return terragruntConfig.Terraform.Source, true
	} else {
		return "", false
	}
}
