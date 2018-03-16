package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/test/helpers"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
)

var mockDefaultInclude = IncludeConfig{Path: DefaultTerragruntConfigPath}

func TestPathRelativeToInclude(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		include           IncludeConfig
		terragruntOptions *options.TerragruntOptions
		expectedPath      string
	}{
		{
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			".",
		},
		{
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			"child",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			"child",
		},
		{
			IncludeConfig{Path: "../../../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			"child/sub-child/sub-sub-child",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			"child/sub-child/sub-sub-child",
		},
		{
			IncludeConfig{Path: "../../other-child/" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/" + DefaultTerragruntConfigPath),
			"../child/sub-child",
		},
		{
			IncludeConfig{Path: "../../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest("../child/sub-child/" + DefaultTerragruntConfigPath),
			"child/sub-child",
		},
		{
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"child/sub-child",
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{include: testCase.include, options: testCase.terragruntOptions}
		actualPath, actualErr := context.pathRelativeToInclude()
		assert.Nil(t, actualErr, "For include %v and options %v, unexpected error: %v", testCase.include, testCase.terragruntOptions, actualErr)
		assert.Equal(t, testCase.expectedPath, actualPath, "For include %v and options %v", testCase.include, testCase.terragruntOptions)
	}
}

func TestPathRelativeFromInclude(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		include           IncludeConfig
		terragruntOptions *options.TerragruntOptions
		expectedPath      string
	}{
		{
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			".",
		},
		{
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			"..",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			"..",
		},
		{
			IncludeConfig{Path: "../../../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			"../../..",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			"../../..",
		},
		{
			IncludeConfig{Path: "../../other-child/" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/" + DefaultTerragruntConfigPath),
			"../../other-child",
		},
		{
			IncludeConfig{Path: "../../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest("../child/sub-child/" + DefaultTerragruntConfigPath),
			"../..",
		},
		{
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"../..",
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{include: testCase.include, options: testCase.terragruntOptions}
		actualPath, actualErr := context.pathRelativeFromInclude()
		assert.Nil(t, actualErr, "For include %v and options %v, unexpected error: %v", testCase.include, testCase.terragruntOptions, actualErr)
		assert.Equal(t, testCase.expectedPath, actualPath, "For include %v and options %v", testCase.include, testCase.terragruntOptions)
	}
}

func TestFindInParentFolders(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		terragruntOptions *options.TerragruntOptions
		expectedPath      string
		expectedErr       error
	}{
		{
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/" + DefaultTerragruntConfigPath),
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			"../../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"",
			parentTerragruntConfigNotFound("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
		},
		{
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/" + DefaultTerragruntConfigPath),
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/sub-child/" + DefaultTerragruntConfigPath),
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.NewTerragruntOptionsForTest("/"),
			"",
			parentTerragruntConfigNotFound("/"),
		},
		{
			options.NewTerragruntOptionsForTest("/fake/path"),
			"",
			parentTerragruntConfigNotFound("/fake/path"),
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{include: mockDefaultInclude, options: testCase.terragruntOptions}
		actualPath, actualErr := context.findInParentFolders()
		if testCase.expectedErr != nil {
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "For options %v, expected error %v but got error %v", testCase.terragruntOptions, testCase.expectedErr, actualErr)
		} else {
			assert.Nil(t, actualErr, "For options %v, unexpected error: %v", testCase.terragruntOptions, actualErr)
			assert.Equal(t, testCase.expectedPath, actualPath, "For options %v", testCase.terragruntOptions)
		}
	}
}

func TestResolveTerragruntInterpolation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		str               string
		include           IncludeConfig
		terragruntOptions *options.TerragruntOptions
		expectedOut       string
		expectedErr       error
	}{
		{
			"${path_relative_to_include()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			".",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"child",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"child/sub-child",
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"",
			parentTerragruntConfigNotFound("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
		},
		{
			"${find_in_parent_folders}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"",
			invalidInterpolationSyntax("${find_in_parent_folders}"),
		},
		{
			"{find_in_parent_folders()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"",
			invalidInterpolationSyntax("{find_in_parent_folders()}"),
		},
		{
			"find_in_parent_folders()",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"",
			invalidInterpolationSyntax("find_in_parent_folders()"),
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{include: testCase.include, options: testCase.terragruntOptions}
		actualOut, actualErr := context.resolveTerragruntInterpolation(testCase.str)
		if testCase.expectedErr != nil {
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "For string '%s' include %v and options %v, expected error %v but got error %v", testCase.str, testCase.include, testCase.terragruntOptions, testCase.expectedErr, actualErr)
		} else {
			assert.Nil(t, actualErr, "For string '%s' include %v and options %v, unexpected error: %v", testCase.str, testCase.include, testCase.terragruntOptions, actualErr)
			assert.Equal(t, testCase.expectedOut, actualOut, "For string '%s' include %v and options %v", testCase.str, testCase.include, testCase.terragruntOptions)
		}
	}
}

func TestResolveTerragruntConfigString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		str               string
		include           IncludeConfig
		terragruntOptions *options.TerragruntOptions
		expectedOut       string
		expectedErr       error
	}{
		{
			"",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"",
			nil,
		},
		{
			"foo bar",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"foo bar",
			nil,
		},
		{
			"$foo {bar}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"$foo {bar}",
			nil,
		},
		{
			"${path_relative_to_include()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			".",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"child",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"child/sub-child",
			nil,
		},
		{
			"foo/${path_relative_to_include()}/bar",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"foo/./bar",
			nil,
		},

		{
			"foo/${path_relative_to_include()}/bar",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"foo/child/bar",
			nil,
		},
		{
			"foo/${path_relative_to_include()}/bar/${path_relative_to_include()}",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"foo/child/bar/child",
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			"${    find_in_parent_folders()    }",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			"${find_in_parent_folders ()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"",
			invalidInterpolationSyntax("${find_in_parent_folders ()}"),
		},
		{
			"foo/${find_in_parent_folders()}/bar",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			fmt.Sprintf("foo/../../%s/bar", DefaultTerragruntConfigPath),
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			"",
			parentTerragruntConfigNotFound("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
		},
		{
			"foo/${unknown}/bar",
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath),
			"",
			invalidInterpolationSyntax("${unknown}"),
		},
	}

	for _, testCase := range testCases {
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, testCase.include, testCase.terragruntOptions)
		if testCase.expectedErr != nil {
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "For string '%s' include %v and options %v, expected error %v but got error %v", testCase.str, testCase.include, testCase.terragruntOptions, testCase.expectedErr, actualErr)
		} else {
			assert.Nil(t, actualErr, "For string '%s' include %v and options %v, unexpected error: %v", testCase.str, testCase.include, testCase.terragruntOptions, actualErr)
			assert.Equal(t, testCase.expectedOut, actualOut, "For string '%s' include %v and options %v", testCase.str, testCase.include, testCase.terragruntOptions)
		}
	}
}

func TestResolveEnvInterpolationConfigString(t *testing.T) {
	t.Parallel()

	defaultOptions := options.NewTerragruntOptionsForTest("/root/child/" + DefaultTerragruntConfigPath)

	optionsWithEnv := func(env map[string]string) *options.TerragruntOptions {
		newOptions := defaultOptions.Clone(defaultOptions.TerragruntConfigPath)
		newOptions.Env = env
		return newOptions
	}

	testCases := []struct {
		str               string
		terragruntOptions *options.TerragruntOptions
		expectedOut       string
		expectedErr       error
	}{
		{"foo/${get_env()}/bar", defaultOptions, "", invalidGetEnvParameters("")},
		{"foo/${get_env(Invalid Parameters)}/bar", defaultOptions, "", invalidInterpolationSyntax("${get_env(Invalid Parameters)}")},
		{"foo/${get_env('env','')}/bar", defaultOptions, "", invalidInterpolationSyntax("${get_env('env','')}")},
		{`foo/${get_env("","")}/bar`, defaultOptions, "", invalidGetEnvParameters(`"",""`)},
		{`foo/${get_env(   ""    ,   ""    )}/bar`, defaultOptions, "", invalidGetEnvParameters(`   ""    ,   ""    `)},
		{`${get_env("SOME_VAR", "SOME{VALUE}")}`, defaultOptions, "SOME{VALUE}", nil},
		{
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT","")}/bar`,
			optionsWithEnv(map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"}),
			"foo//bar",
			nil,
		},
		{
			`foo/${get_env(    "TEST_ENV_TERRAGRUNT_HIT"   ,   ""   )}/bar`,
			optionsWithEnv(map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"}),
			"foo//bar",
			nil,
		},
		{
			`foo/${get_env("TEST_ENV_
TERRAGRUNT_HIT","")}/bar`,
			optionsWithEnv(map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"}),
			"",
			invalidInterpolationSyntax(`${get_env("TEST_ENV_
TERRAGRUNT_HIT","")}`),
		},
		{
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT","DEFAULT")}/bar`,
			optionsWithEnv(map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"}),
			"foo/DEFAULT/bar",
			nil,
		},
		{
			`foo/${get_env(    "TEST_ENV_TERRAGRUNT_HIT      "   ,   "default"   )}/bar`,
			optionsWithEnv(map[string]string{"TEST_ENV_TERRAGRUNT_HIT": "environment hit  "}),
			"foo/environment hit  /bar",
			nil,
		},
		{
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT","default")}/bar`,
			optionsWithEnv(map[string]string{"TEST_ENV_TERRAGRUNT_HIT": "HIT"}),
			"foo/HIT/bar",
			nil,
		},
		// Unclosed quote
		{`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT}/bar`, defaultOptions, "", invalidInterpolationSyntax(`${get_env("TEST_ENV_TERRAGRUNT_HIT}`)},
		// Unclosed quote and interpolation pattern
		{`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT/bar`, defaultOptions, `foo/${get_env("TEST_ENV_TERRAGRUNT_HIT/bar`, nil},
	}

	for _, testCase := range testCases {
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, mockDefaultInclude, testCase.terragruntOptions)
		if testCase.expectedErr != nil {
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "For string '%s' options %v, expected error %v but got error %v and output %v", testCase.str, testCase.terragruntOptions, testCase.expectedErr, actualErr, actualOut)
		} else {
			assert.Nil(t, actualErr, "For string '%s' options %v, unexpected error: %v", testCase.str, testCase.terragruntOptions, actualErr)
			assert.Equal(t, testCase.expectedOut, actualOut, "For string '%s' options %v", testCase.str, testCase.terragruntOptions)
		}
	}
}

func TestResolveCommandsInterpolationConfigString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		str         string
		expectedOut string
		expectedErr error
	}{
		{
			`"${get_terraform_commands_that_need_locking()}"`,
			util.CommaSeparatedStrings(TerraformCommandWithLockTimeout),
			nil,
		},
		{
			`commands = ["${get_terraform_commands_that_need_vars()}"]`,
			fmt.Sprintf("commands = [%s]", util.CommaSeparatedStrings(TerraformCommandWithVarFile)),
			nil,
		},
		{
			`commands = "test-${get_terraform_commands_that_need_vars()}"`,
			fmt.Sprintf(`commands = "test-%v"`, TerraformCommandWithVarFile),
			nil,
		},
	}

	for _, testCase := range testCases {
		options := options.NewTerragruntOptionsForTest(DefaultTerragruntConfigPath)
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, mockDefaultInclude, options)
		if testCase.expectedErr != nil {
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "For string '%s', expected error %v but got error %v and output %v", testCase.str, testCase.expectedErr, actualErr, actualOut)
		} else {
			assert.Nil(t, actualErr, "For string '%s', unexpected error: %v", testCase.str, actualErr)
			assert.Equal(t, testCase.expectedOut, actualOut, "For string '%s'", testCase.str)
		}
	}
}

func TestResolveMultipleInterpolationsConfigString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		str         string
		expectedOut string
		expectedErr error
	}{
		{
			`${get_env("NON_EXISTING_VAR1", "default1")}-${get_env("NON_EXISTING_VAR2", "default2")}`,
			fmt.Sprintf("default1-default2"),
			nil,
		},
		{
			// Included within quotes
			`"${get_env("NON_EXISTING_VAR1", "default1")}-${get_env("NON_EXISTING_VAR2", "default2")}"`,
			`"default1-default2"`,
			nil,
		},
		{
			// Malformed parameters
			`${get_env("NON_EXISTING_VAR1", "default"-${get_terraform_commands_that_need_vars()}`,
			fmt.Sprintf(`${get_env("NON_EXISTING_VAR1", "default"-%v`, TerraformCommandWithVarFile),
			nil,
		},
		{
			`test1 = "${get_env("NON_EXISTING_VAR1", "default")}" test2 = ["${get_terraform_commands_that_need_vars()}"]`,
			fmt.Sprintf(`test1 = "default" test2 = [%v]`, util.CommaSeparatedStrings(TerraformCommandWithVarFile)),
			nil,
		},
		{
			`${get_env("NON_EXISTING_VAR1", "default")}-${get_terraform_commands_that_need_vars()}`,
			fmt.Sprintf("default-%v", TerraformCommandWithVarFile),
			nil,
		},
	}

	for _, testCase := range testCases {
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, mockDefaultInclude, options.NewTerragruntOptionsForTest(DefaultTerragruntConfigPath))
		if testCase.expectedErr != nil {
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "For string '%s', expected error %v but got error %v and output %v", testCase.str, testCase.expectedErr, actualErr, actualOut)
		} else {
			assert.Nil(t, actualErr, "For string '%s', unexpected error: %v", testCase.str, actualErr)
			assert.Equal(t, testCase.expectedOut, actualOut, "For string '%s'", testCase.str)
		}
	}
}

func TestGetTfVarsDirAbsPath(t *testing.T) {
	t.Parallel()
	workingDir, err := os.Getwd()
	assert.Nil(t, err, "Could not get current working dir: %v", err)
	testGetTfVarsDir(t, "/foo/bar/terraform.tfvars", fmt.Sprintf("%s/foo/bar", filepath.VolumeName(workingDir)))
}

func TestGetTfVarsDirRelPath(t *testing.T) {
	t.Parallel()
	workingDir, err := os.Getwd()
	assert.Nil(t, err, "Could not get current working dir: %v", err)
	workingDir = filepath.ToSlash(workingDir)

	testGetTfVarsDir(t, "foo/bar/terraform.tfvars", fmt.Sprintf("%s/foo/bar", workingDir))
}

func testGetTfVarsDir(t *testing.T, configPath string, expectedPath string) {
	context := resolveContext{include: mockDefaultInclude, options: options.NewTerragruntOptionsForTest(configPath)}
	actualPath, err := context.getTfVarsDir()

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expectedPath, actualPath)
}

func TestGetParentTfVarsDir(t *testing.T) {
	t.Parallel()

	currentDir, err := os.Getwd()
	assert.Nil(t, err, "Could not get current working dir: %v", err)
	parentDir := filepath.ToSlash(filepath.Dir(currentDir))

	testCases := []struct {
		include           IncludeConfig
		terragruntOptions *options.TerragruntOptions
		expectedPath      string
	}{
		{
			mockDefaultInclude,
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			helpers.RootFolder + "child",
		},
		{
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/" + DefaultTerragruntConfigPath),
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: "../../../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath),
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: "../../other-child/" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest(helpers.RootFolder + "child/sub-child/" + DefaultTerragruntConfigPath),
			fmt.Sprintf("%s/other-child", filepath.VolumeName(parentDir)),
		},
		{
			IncludeConfig{Path: "../../" + DefaultTerragruntConfigPath},
			options.NewTerragruntOptionsForTest("../child/sub-child/" + DefaultTerragruntConfigPath),
			parentDir,
		},
		{
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
			fmt.Sprintf("%s/test/fixture-parent-folders/terragrunt-in-root", parentDir),
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{include: testCase.include, options: testCase.terragruntOptions}
		actualPath, actualErr := context.getParentTfVarsDir()
		assert.Nil(t, actualErr, "For include %v and options %v, unexpected error: %v", testCase.include, testCase.terragruntOptions, actualErr)
		assert.Equal(t, testCase.expectedPath, actualPath, "For include %v and options %v", testCase.include, testCase.terragruntOptions)
	}
}

func Test_getParameters(t *testing.T) {
	type args struct {
		parameters        string
		regex             *regexp.Regexp
		terragruntOptions *options.TerragruntOptions
	}

	mockOptions := options.NewTerragruntOptionsForTest("")
	mockOptions.Variables = map[string]options.Variable{
		"a": {Source: options.Default, Value: "a"},
		"b": {Source: options.Default, Value: "b"},
	}

	tests := []struct {
		name             string
		errorOnUndefined bool
		args             args
		wantResult       []string
		wantErr          bool
	}{
		{"Normal case", true, args{`var.a, var.b, "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"a", "b", "text"}, false},
		{"Var with -", true, args{`var.a-1, var.b, "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"", "b", "text"}, false},
		{"Var with - Keep undefined", false, args{`var.a-1, var.b, "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"${var.a-1}", "b", "text"}, false},
		{"Too much parameters", true, args{`var.a, var.b, "text", var.c`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, nil, true},
		{"With function", true, args{`var.a-1, default(var.a, "no a"), "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"", "a", "text"}, false},
		{"With default no value", false, args{`"", default(var.c, "no c"), "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"", "no c", "text"}, false},
		{"Wrong number of parameters", true, args{`var.a-1, default(var.c, "no c", extra), "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.terragruntOptions.IgnoreRemainingInterpolation = !tt.errorOnUndefined
			context := resolveContext{
				include:    mockDefaultInclude,
				options:    tt.args.terragruntOptions,
				parameters: tt.args.parameters,
			}
			gotResult, err := context.getParameters(tt.args.regex)
			if (err != nil) != tt.wantErr {
				t.Errorf("getParameters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("getParameters() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
