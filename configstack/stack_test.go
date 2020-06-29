package configstack

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/stretchr/testify/assert"
)

func TestFindStackInSubfolders(t *testing.T) {
	t.Parallel()

	filePaths := []string{
		"/stage/data-stores/redis/" + config.DefaultConfigName,
		"/stage/data-stores/postgres/" + config.DefaultConfigName,
		"/stage/ecs-cluster/" + config.DefaultConfigName,
		"/stage/kms-master-key/" + config.DefaultConfigName,
		"/stage/vpc/" + config.DefaultConfigName,
	}

	tempFolder := createTempFolder(t)
	writeDummyTerragruntConfigs(t, tempFolder, filePaths)

	envFolder := filepath.ToSlash(util.JoinPath(tempFolder + "/stage"))
	terragruntOptions := options.NewTerragruntOptions(envFolder)
	terragruntOptions.WorkingDir = envFolder

	stack, err := FindStackInSubfolders(terragruntOptions)
	if err != nil {
		t.Fatalf("Failed when calling method under test: %s\n", err.Error())
	}

	var modulePaths []string

	for _, module := range stack.Modules {
		relPath := strings.Replace(module.Path, tempFolder, "", 1)
		relPath = filepath.ToSlash(util.JoinPath(relPath, config.DefaultConfigName))

		modulePaths = append(modulePaths, relPath)
	}

	for _, filePath := range filePaths {
		filePathFound := util.ListContainsElement(modulePaths, filePath)
		assert.True(t, filePathFound, "The filePath %s was not found by Terragrunt.\n", filePath)
	}
}

func createTempFolder(t *testing.T) string {
	tmpFolder, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %s\n", err.Error())
	}

	return filepath.ToSlash(tmpFolder)
}

// Create a dummy Terragrunt config file at each of the given paths
func writeDummyTerragruntConfigs(t *testing.T, tmpFolder string, paths []string) {
	contents := []byte("terraform {\nsource = \"test\"\n}")
	for _, path := range paths {
		absPath := util.JoinPath(tmpFolder, path)

		containingDir := filepath.Dir(absPath)
		createDirIfNotExist(t, containingDir)

		err := ioutil.WriteFile(absPath, contents, os.ModePerm)
		if err != nil {
			t.Fatalf("Failed to write file at path %s: %s\n", path, err.Error())
		}
	}
}

func createDirIfNotExist(t *testing.T, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			t.Fatalf("Failed to create directory: %s\n", err.Error())
		}
	}
}
