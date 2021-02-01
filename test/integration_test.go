package test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/terragrunt/v2/awshelper"
	"github.com/coveooss/terragrunt/v2/cli"
	"github.com/coveooss/terragrunt/v2/config"
	terragruntDynamoDb "github.com/coveooss/terragrunt/v2/dynamodb"
	"github.com/coveooss/terragrunt/v2/remote"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/stretchr/testify/assert"
)

const terraformRemoteStateS3Region = "us-west-2"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func trim(s string) string { return fmt.Sprintln(strings.TrimSpace(collections.UnIndent(s))) }

func TestTerragruntWorksWithLocalTerraformVersion(t *testing.T) {
	t.Parallel()

	const testFixturePath = "fixture/"
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	cleanupTerraformFolder(t, testFixturePath)

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))
	lockTableName := fmt.Sprintf("terragrunt-test-locks-%s", strings.ToLower(uniqueID()))

	tmpTerragruntConfigPath := createTmpTerragruntConfig(t, testFixturePath, s3BucketName, lockTableName, config.DefaultConfigName)

	defer deleteS3Bucket(t, terraformRemoteStateS3Region, s3BucketName)
	defer cleanupTableForTest(t, lockTableName, terraformRemoteStateS3Region)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-config %s --terragrunt-working-dir %s", tmpTerragruntConfigPath, testFixturePath))
	validateS3BucketExists(t, terraformRemoteStateS3Region, s3BucketName)
}

func TestTerragruntWorksWithIncludes(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-include/"
	const relative = "qa/my-app"
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	childPath := util.JoinPath(testPath, relative)
	cleanupTerraformFolder(t, childPath)

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))

	tmpTerragruntConfigPath := createTmpTerragruntConfigWithParentAndChild(t, testPath, relative, s3BucketName, config.DefaultConfigName, config.DefaultConfigName)

	defer deleteS3Bucket(t, terraformRemoteStateS3Region, s3BucketName)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-config %s --terragrunt-working-dir %s", tmpTerragruntConfigPath, childPath))
}

func TestTerragruntOutputAllCommand(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-output-all"
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))

	defer deleteS3Bucket(t, terraformRemoteStateS3Region, s3BucketName)

	tmpEnvPath := copyEnvironment(t, testPath)

	rootTerragruntConfigPath := util.JoinPath(tmpEnvPath, testPath, config.DefaultConfigName)
	copyTerragruntConfigAndFillPlaceholders(t, rootTerragruntConfigPath, rootTerragruntConfigPath, s3BucketName, "not-used")

	environmentPath := fmt.Sprintf("%s/%s/env1", tmpEnvPath, testPath)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply-all --terragrunt-non-interactive --terragrunt-working-dir %s", environmentPath))

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt output-all --terragrunt-non-interactive --terragrunt-working-dir %s", environmentPath), &stdout, &stderr)
	output := stdout.String()

	assert.True(t, strings.Contains(output, "app1 output"))
	assert.True(t, strings.Contains(output, "app2 output"))
	assert.True(t, strings.Contains(output, "app3 output"))

	assert.True(t, (strings.Index(output, "app3 output") < strings.Index(output, "app1 output")) && (strings.Index(output, "app1 output") < strings.Index(output, "app2 output")))
}

func TestTerragruntOutputAllCommandSpecificVariableIgnoreDependencyErrors(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-output-all"
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))

	defer deleteS3Bucket(t, terraformRemoteStateS3Region, s3BucketName)

	tmpEnvPath := copyEnvironment(t, testPath)

	rootTerragruntConfigPath := util.JoinPath(tmpEnvPath, testPath, config.DefaultConfigName)
	copyTerragruntConfigAndFillPlaceholders(t, rootTerragruntConfigPath, rootTerragruntConfigPath, s3BucketName, "not-used")

	environmentPath := fmt.Sprintf("%s/%s/env1", tmpEnvPath, testPath)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply-all --terragrunt-non-interactive --terragrunt-working-dir %s", environmentPath))

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	// Call runTerragruntCommand directly because this command contains failures (which causes runTerragruntRedirectOutput to abort) but we don't care.
	runTerragruntCommand(t, fmt.Sprintf("terragrunt output-all app2_text --terragrunt-ignore-dependency-errors --terragrunt-non-interactive --terragrunt-working-dir %s", environmentPath), &stdout, &stderr)
	output := stdout.String()

	// Without --terragrunt-ignore-dependency-errors, app2 never runs because its dependencies have "errors" since they don't have the output "app2_text".
	assert.True(t, strings.Contains(output, "app2 output"))
}

func TestTerragruntStackCommands(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-stack/"
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))
	lockTableName := fmt.Sprintf("terragrunt-test-locks-%s", strings.ToLower(uniqueID()))

	tmpEnvPath := copyEnvironment(t, testPath)

	rootTerragruntConfigPath := util.JoinPath(tmpEnvPath, "fixture-stack", config.DefaultConfigName)
	copyTerragruntConfigAndFillPlaceholders(t, rootTerragruntConfigPath, rootTerragruntConfigPath, s3BucketName, lockTableName)

	mgmtEnvironmentPath := fmt.Sprintf("%s/fixture-stack/mgmt", tmpEnvPath)
	stageEnvironmentPath := fmt.Sprintf("%s/fixture-stack/stage", tmpEnvPath)

	defer deleteS3Bucket(t, terraformRemoteStateS3Region, s3BucketName)
	defer cleanupTableForTest(t, lockTableName, terraformRemoteStateS3Region)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply-all --terragrunt-non-interactive --terragrunt-working-dir %s", mgmtEnvironmentPath))
	runTerragrunt(t, fmt.Sprintf("terragrunt apply-all --terragrunt-non-interactive --terragrunt-working-dir %s", stageEnvironmentPath))

	runTerragrunt(t, fmt.Sprintf("terragrunt output-all --terragrunt-non-interactive --terragrunt-working-dir %s", mgmtEnvironmentPath))
	runTerragrunt(t, fmt.Sprintf("terragrunt output-all --terragrunt-non-interactive --terragrunt-working-dir %s", stageEnvironmentPath))

	runTerragrunt(t, fmt.Sprintf("terragrunt destroy-all --terragrunt-non-interactive --terragrunt-working-dir %s", stageEnvironmentPath))
	runTerragrunt(t, fmt.Sprintf("terragrunt destroy-all --terragrunt-non-interactive --terragrunt-working-dir %s", mgmtEnvironmentPath))
}

func TestTerragruntCustomConfig(t *testing.T) {
	t.Parallel()
	const testPath = "fixture-custom-config/"
	command := fmt.Sprintf("terragrunt get-stack --terragrunt-non-interactive --terragrunt-working-dir %s --terragrunt-config custom-config.hcl", testPath)
	{
		var out, err bytes.Buffer
		runTerragruntRedirectOutput(t, command, &out, &err)
		assert.Equal(t, trim(`
			fixture-custom-config/sub-project-1
			fixture-custom-config/sub-project-2/sub-project-2-1
			fixture-custom-config/sub-project-2/sub-project-2-2
			fixture-custom-config/sub-project-3
		`), out.String())
		assert.Equal(t, "", err.String())
	}
	{ // Outpout yaml
		command := command + " -oy"
		var out, err bytes.Buffer
		runTerragruntRedirectOutput(t, command, &out, &err)
		assert.Equal(t, trim(`
			- path: fixture-custom-config/sub-project-1
			- path: fixture-custom-config/sub-project-2/sub-project-2-1
			  dependencies:
			  - fixture-custom-config/sub-project-1
			- path: fixture-custom-config/sub-project-2/sub-project-2-2
			  dependencies:
			  - fixture-custom-config/sub-project-2/sub-project-2-1
			  - fixture-custom-config/sub-project-1
			- path: fixture-custom-config/sub-project-3
			  dependencies:
			  - fixture-custom-config/sub-project-2/sub-project-2-1
			  - fixture-custom-config/sub-project-2/sub-project-2-2
			  - fixture-custom-config/sub-project-1
		`)+"\n", out.String())
		assert.Equal(t, "", err.String())
	}
	{ // Outpout json
		command := command + " -oj"
		var out, err bytes.Buffer
		runTerragruntRedirectOutput(t, command, &out, &err)
		assert.Equal(t, trim(`
			[
			  {
			    "path": "fixture-custom-config/sub-project-1"
			  },
			  {
			    "path": "fixture-custom-config/sub-project-2/sub-project-2-1",
			    "dependencies": [
			      "fixture-custom-config/sub-project-1"
			    ]
			  },
			  {
			    "path": "fixture-custom-config/sub-project-2/sub-project-2-2",
			    "dependencies": [
			      "fixture-custom-config/sub-project-2/sub-project-2-1",
			      "fixture-custom-config/sub-project-1"
			    ]
			  },
			  {
			    "path": "fixture-custom-config/sub-project-3",
			    "dependencies": [
			      "fixture-custom-config/sub-project-2/sub-project-2-1",
			      "fixture-custom-config/sub-project-2/sub-project-2-2",
			      "fixture-custom-config/sub-project-1"
			    ]
			  }
			]		
		`), out.String())
		assert.Equal(t, "", err.String())
	}
	{ // Outpout hcl
		command := command + " -oh"
		var out, err bytes.Buffer
		runTerragruntRedirectOutput(t, command, &out, &err)
		assert.Equal(t, trim(`
			[
			  {
			    path = "fixture-custom-config/sub-project-1"
			  },
			  {
			    dependencies = ["fixture-custom-config/sub-project-1"]
			    path         = "fixture-custom-config/sub-project-2/sub-project-2-1"
			  },
			  {
			    path = "fixture-custom-config/sub-project-2/sub-project-2-2"
			    
			    dependencies = [
			      "fixture-custom-config/sub-project-2/sub-project-2-1",
			      "fixture-custom-config/sub-project-1",
			    ]
			  },
			  {
			    path = "fixture-custom-config/sub-project-3"
			    
			    dependencies = [
			      "fixture-custom-config/sub-project-2/sub-project-2-1",
			      "fixture-custom-config/sub-project-2/sub-project-2-2",
			      "fixture-custom-config/sub-project-1",
			    ]
			  },
			]
		`), out.String())
		assert.Equal(t, "", err.String())
	}
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name string
		path string
		args []string
	}{
		{"local", "fixture-download/local", nil},
		{"local-relative", "fixture-download/local-relative", nil},
		{"hidden-folder", "fixture-download/local-with-hidden-folder", nil},
		{"remote", "fixture-download/remote", nil},
		{"remote-relative", "fixture-download/remote-relative", nil},
		{"override", "fixture-download/override", []string{"--terragrunt-source ../hello-world"}},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			command := strings.Join(append([]string{fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", tt.path)}, tt.args...), " ")
			runTerragrunt(t, command+" --terragrunt-source-update")
			// Run a second time to make sure the temporary folder can be reused without errors
			runTerragrunt(t, command)
		})
	}
}

func TestLocalWithBackend(t *testing.T) {
	t.Parallel()
	const testPath = "fixture-download/local-with-backend"
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))
	lockTableName := fmt.Sprintf("terragrunt-lock-table-%s", strings.ToLower(uniqueID()))
	tmpEnvPath := copyEnvironment(t, "fixture-download")
	rootPath := util.JoinPath(tmpEnvPath, testPath)
	rootTerragruntConfigPath := util.JoinPath(rootPath, config.DefaultConfigName)
	copyTerragruntConfigAndFillPlaceholders(t, rootTerragruntConfigPath, rootTerragruntConfigPath, s3BucketName, lockTableName)

	defer deleteS3Bucket(t, terraformRemoteStateS3Region, s3BucketName)
	defer cleanupTableForTest(t, lockTableName, terraformRemoteStateS3Region)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))
	// Run a second time to make sure the temporary folder can be reused without errors
	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))
}

func TestRemoteWithBackend(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-download/remote-with-backend"
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	s3BucketName := fmt.Sprintf("terragrunt-test-bucket-%s", strings.ToLower(uniqueID()))
	lockTableName := fmt.Sprintf("terragrunt-lock-table-%s", strings.ToLower(uniqueID()))
	tmpEnvPath := copyEnvironment(t, testPath)
	rootPath := util.JoinPath(tmpEnvPath, testPath)
	rootTerragruntConfigPath := util.JoinPath(rootPath, config.DefaultConfigName)
	copyTerragruntConfigAndFillPlaceholders(t, rootTerragruntConfigPath, rootTerragruntConfigPath, s3BucketName, lockTableName)

	defer deleteS3Bucket(t, terraformRemoteStateS3Region, s3BucketName)
	defer cleanupTableForTest(t, lockTableName, terraformRemoteStateS3Region)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))
	// Run a second time to make sure the temporary folder can be reused without errors
	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath))
}

func TestExtraArguments(t *testing.T) {
	// Do not use t.Parallel() on this testPath, it will infers with the other TestExtraArguments.* tests
	const testPath = "fixture-extra-args/"
	out := new(bytes.Buffer)
	tmpEnvPath := copyEnvironment(t, testPath) + "/" + testPath
	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", tmpEnvPath), out, os.Stderr)
	t.Log(out.String())
	assert.Contains(t, out.String(), "Hello, World from dev!")
}

func TestExtraArgumentsWithEnv(t *testing.T) {
	// Do not use t.Parallel() on this testPath, it will infers with the other TestExtraArguments.* tests
	const testPath = "fixture-extra-args/"
	out := new(bytes.Buffer)
	tmpEnvPath := copyEnvironment(t, testPath) + "/" + testPath
	os.Setenv("TF_VAR_env", "prod")
	defer os.Unsetenv("TF_VAR_env")
	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", tmpEnvPath), out, os.Stderr)
	t.Log(out.String())
	assert.Contains(t, out.String(), "Hello, World!")
}

func TestExtraArgumentsWithRegion(t *testing.T) {
	// Do not use t.Parallel() on this testPath, it will infers with the other TestExtraArguments.* tests
	const testPath = "fixture-extra-args/"
	out := new(bytes.Buffer)
	tmpEnvPath := copyEnvironment(t, testPath) + "/" + testPath
	os.Setenv("TF_VAR_region", "us-west-2")
	defer os.Unsetenv("TF_VAR_region")
	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", tmpEnvPath), out, os.Stderr)
	t.Log(out.String())
	assert.Contains(t, out.String(), "Hello, World from Oregon!")
}

func TestPriorityOrderOfArgument(t *testing.T) {
	// Do not use t.Parallel() on this testPath, it will infers with the other TestExtraArguments.* tests
	const testPath = "fixture-extra-args/"
	out := new(bytes.Buffer)
	tmpEnvPath := copyEnvironment(t, testPath) + "/" + testPath
	injectedValue := "Injected-directly-by-argument"
	runTerragruntRedirectOutput(t, fmt.Sprintf("terragrunt apply -var extra_var=%s --terragrunt-non-interactive --terragrunt-working-dir %s", injectedValue, tmpEnvPath), out, os.Stderr)
	t.Log(out.String())
	// And the result value for test should be the injected variable since the injected arguments are injected before the suplied parameters,
	// so our override of extra_var should be the last argument.
	assert.Contains(t, out.String(), fmt.Sprintf(`test = "%s"`, injectedValue))
}

func cleanupTerraformFolder(t *testing.T, templatesPath string) {
	removeFile(t, util.JoinPath(templatesPath, "terraform.tfstate"))
	removeFile(t, util.JoinPath(templatesPath, "terraform.tfstate.backup"))
	removeFolder(t, util.JoinPath(templatesPath, ".terraform"))
}

func removeFile(t *testing.T, path string) {
	if util.FileExists(path) {
		if err := os.Remove(path); err != nil {
			t.Fatalf("Error while removing %s: %v", path, err)
		}
	}
}

func removeFolder(t *testing.T, path string) {
	if util.FileExists(path) {
		if err := os.RemoveAll(path); err != nil {
			t.Fatalf("Error while removing %s: %v", path, err)
		}
	}
}

func runTerragruntCommand(t *testing.T, command string, writer io.Writer, errwriter io.Writer) error {
	args := util.RemoveElementFromList(strings.Split(command, " "), "")
	app := cli.CreateTerragruntCli("TEST", writer, errwriter)
	return app.Run(args)
}

func runTerragrunt(t *testing.T, command string) {
	runTerragruntRedirectOutput(t, command, os.Stdout, os.Stderr)
}

func runTerragruntRedirectOutput(t *testing.T, command string, writer, errwriter io.Writer) {
	if err := runTerragruntCommand(t, command, writer, errwriter); err != nil {
		message := ""
		if err, captured := errwriter.(*bytes.Buffer); captured {
			message = "\n" + err.String()
		}
		t.Errorf("Failed to run Terragrunt command '%s' due to error: %v%s", command, err, message)
	}
}

func withEnv(variables string, testFunction func()) {
	// We take a copy of the current environment variables
	env := os.Environ()

	// We create a map of environment variable that should be kept
	keptVariables := make(map[string]string)
	for _, env := range strings.Split(variables, ",") {
		keptVariables[env] = os.Getenv(env)
	}

	// We clear all environment variables and restore only the variables specified by variables
	os.Clearenv()
	for key, value := range keptVariables {
		os.Setenv(key, value)
	}

	// We register a deferred function to restore back the original environment variables
	defer func() {
		for i := range env {
			values := strings.SplitN(env[i], "=", 2)
			os.Setenv(values[0], values[1])
		}
	}()

	testFunction()
}

func copyEnvironment(t *testing.T, environmentPath string) string {
	tmpDir, err := ioutil.TempDir("", "terragrunt-stack-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir due to error: %v", err)
	}

	t.Logf("Copying %s to %s", environmentPath, tmpDir)

	err = filepath.Walk(environmentPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		destPath := util.JoinPath(tmpDir, path)

		destPathDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destPathDir, 0777); err != nil {
			return err
		}

		return copyFile(path, destPath)
	})

	if err != nil {
		t.Fatalf("Error walking file path %s due to error: %v", environmentPath, err)
	}

	return tmpDir
}

func copyFile(srcPath string, destPath string) error {
	contents, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(destPath, contents, 0644)
}

func createTmpTerragruntConfigWithParentAndChild(t *testing.T, parentPath string, childRelPath string, s3BucketName string, parentConfigFileName string, childConfigFileName string) string {
	tmpDir, err := ioutil.TempDir("", "terragrunt-parent-child-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir due to error: %v", err)
	}

	childDestPath := util.JoinPath(tmpDir, childRelPath)

	if err := os.MkdirAll(childDestPath, 0777); err != nil {
		t.Fatalf("Failed to create temp dir %s due to error %v", childDestPath, err)
	}

	parentTerragruntSrcPath := util.JoinPath(parentPath, parentConfigFileName)
	parentTerragruntDestPath := util.JoinPath(tmpDir, parentConfigFileName)
	copyTerragruntConfigAndFillPlaceholders(t, parentTerragruntSrcPath, parentTerragruntDestPath, s3BucketName, "not-used")

	childTerragruntSrcPath := util.JoinPath(util.JoinPath(parentPath, childRelPath), childConfigFileName)
	childTerragruntDestPath := util.JoinPath(childDestPath, childConfigFileName)
	copyTerragruntConfigAndFillPlaceholders(t, childTerragruntSrcPath, childTerragruntDestPath, s3BucketName, "not-used")

	return childTerragruntDestPath
}

func createTmpTerragruntConfig(t *testing.T, templatesPath string, s3BucketName string, lockTableName string, configFileName string) string {
	tmpFolder, err := ioutil.TempDir("", "terragrunt-test")
	if err != nil {
		t.Fatalf("Failed to create temp folder due to error: %v", err)
	}

	tmpTerragruntConfigFile := util.JoinPath(tmpFolder, configFileName)
	originalTerragruntConfigPath := util.JoinPath(templatesPath, configFileName)
	copyTerragruntConfigAndFillPlaceholders(t, originalTerragruntConfigPath, tmpTerragruntConfigFile, s3BucketName, lockTableName)

	return tmpTerragruntConfigFile
}

func copyTerragruntConfigAndFillPlaceholders(t *testing.T, configSrcPath string, configDestPath string, s3BucketName string, lockTableName string) {
	contents, err := util.ReadFileAsString(configSrcPath)
	if err != nil {
		t.Fatalf("Error reading Terragrunt config at %s: %v", configSrcPath, err)
	}

	contents = strings.Replace(contents, "__FILL_IN_BUCKET_NAME__", s3BucketName, -1)
	contents = strings.Replace(contents, "__FILL_IN_LOCK_TABLE_NAME__", lockTableName, -1)

	if err := ioutil.WriteFile(configDestPath, []byte(contents), 0444); err != nil {
		t.Fatalf("Error writing temp Terragrunt config to %s: %v", configDestPath, err)
	}
}

// Returns a unique (ish) id we can attach to resources and tfstate files so they don't conflict with each other
// Uses base 62 to generate a 6 character string that's unlikely to collide with the handful of tests we run in
// parallel. Based on code here: http://stackoverflow.com/a/9543797/483528
func uniqueID() string {
	const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const uniqueIDLength = 6 // Should be good for 62^6 = 56+ billion combinations

	var out bytes.Buffer

	for i := 0; i < uniqueIDLength; i++ {
		out.WriteByte(base62Chars[rand.Intn(len(base62Chars))])
	}

	return out.String()
}

// Check that the S3 Bucket of the given name and region exists. Terragrunt should create this bucket during the test.
func validateS3BucketExists(t *testing.T, awsRegion string, bucketName string) {
	s3Client, err := remote.CreateS3Client(awsRegion, "")
	if err != nil {
		t.Fatalf("Error creating S3 client: %v", err)
	}

	remoteStateConfig := remote.StateConfigS3{Bucket: bucketName, Region: awsRegion}
	assert.True(t, remote.DoesS3BucketExist(s3Client, &remoteStateConfig), "Terragrunt failed to create remote state S3 bucket %s", bucketName)
}

// Delete the specified S3 bucket to clean up after a test
func deleteS3Bucket(t *testing.T, awsRegion string, bucketName string) {
	s3Client, err := remote.CreateS3Client(awsRegion, "")
	if err != nil {
		t.Fatalf("Error creating S3 client: %v", err)
	}

	t.Logf("Deleting test s3 bucket %s", bucketName)

	out, err := s3Client.ListObjectVersions(&s3.ListObjectVersionsInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("Failed to list object versions in s3 bucket %s: %v", bucketName, err)
	}

	objectIdentifiers := []*s3.ObjectIdentifier{}
	for _, version := range out.Versions {
		objectIdentifiers = append(objectIdentifiers, &s3.ObjectIdentifier{
			Key:       version.Key,
			VersionId: version.VersionId,
		})
	}

	if len(objectIdentifiers) > 0 {
		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(bucketName),
			Delete: &s3.Delete{Objects: objectIdentifiers},
		}
		if _, err := s3Client.DeleteObjects(deleteInput); err != nil {
			t.Fatalf("Error deleting all versions of all objects in bucket %s: %v", bucketName, err)
		}
	}

	if _, err := s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)}); err != nil {
		t.Fatalf("Failed to delete S3 bucket %s: %v", bucketName, err)
	}
}

// Create an authenticated client for DynamoDB
func CreateDynamoDbClient(awsRegion, awsProfile string) (*dynamodb.DynamoDB, error) {
	session, err := awshelper.CreateAwsSession(awsRegion, awsProfile)
	if err != nil {
		return nil, err
	}

	return dynamodb.New(session), nil
}

func createDynamoDbClientForTest(t *testing.T, awsRegion string) *dynamodb.DynamoDB {
	client, err := CreateDynamoDbClient(awsRegion, "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func cleanupTableForTest(t *testing.T, tableName string, awsRegion string) {
	client := createDynamoDbClientForTest(t, awsRegion)
	err := terragruntDynamoDb.DeleteTable(tableName, client)
	assert.Nil(t, err, "Unexpected error: %v", err)
}
