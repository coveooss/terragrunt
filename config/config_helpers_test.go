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
		terragruntOptions options.TerragruntOptions
		expectedPath      string
	}{
		{
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			".",
		},
		{
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child",
		},
		{
			IncludeConfig{Path: "../../../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child/sub-child/sub-sub-child",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child/sub-child/sub-sub-child",
		},
		{
			IncludeConfig{Path: "../../other-child/" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../child/sub-child",
		},
		{
			IncludeConfig{Path: "../../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: "../child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child/sub-child",
		},
		{
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child/sub-child",
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{testCase.include, &testCase.terragruntOptions, ""}
		actualPath, actualErr := context.pathRelativeToInclude()
		assert.Nil(t, actualErr, "For include %v and options %v, unexpected error: %v", testCase.include, testCase.terragruntOptions, actualErr)
		assert.Equal(t, testCase.expectedPath, actualPath, "For include %v and options %v", testCase.include, testCase.terragruntOptions)
	}
}

func TestPathRelativeFromInclude(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		include           IncludeConfig
		terragruntOptions options.TerragruntOptions
		expectedPath      string
	}{
		{
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			".",
		},
		{
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"..",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"..",
		},
		{
			IncludeConfig{Path: "../../../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../../..",
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../../..",
		},
		{
			IncludeConfig{Path: "../../other-child/" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../../other-child",
		},
		{
			IncludeConfig{Path: "../../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: "../child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../..",
		},
		{
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../..",
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{testCase.include, &testCase.terragruntOptions, ""}
		actualPath, actualErr := context.pathRelativeFromInclude()
		assert.Nil(t, actualErr, "For include %v and options %v, unexpected error: %v", testCase.include, testCase.terragruntOptions, actualErr)
		assert.Equal(t, testCase.expectedPath, actualPath, "For include %v and options %v", testCase.include, testCase.terragruntOptions)
	}
}

func TestFindInParentFolders(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		terragruntOptions options.TerragruntOptions
		expectedPath      string
		expectedErr       error
	}{
		{
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			ParentTerragruntConfigNotFound("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
		},
		{
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			options.TerragruntOptions{TerragruntConfigPath: "/", NonInteractive: true},
			"",
			ParentTerragruntConfigNotFound("/"),
		},
		{
			options.TerragruntOptions{TerragruntConfigPath: "/fake/path", NonInteractive: true},
			"",
			ParentTerragruntConfigNotFound("/fake/path"),
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{mockDefaultInclude, &testCase.terragruntOptions, ""}
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
		terragruntOptions options.TerragruntOptions
		expectedOut       string
		expectedErr       error
	}{
		{
			"${path_relative_to_include()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			".",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child/sub-child",
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			ParentTerragruntConfigNotFound("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
		},
		{
			"${find_in_parent_folders}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax("${find_in_parent_folders}"),
		},
		{
			"{find_in_parent_folders()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax("{find_in_parent_folders()}"),
		},
		{
			"find_in_parent_folders()",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax("find_in_parent_folders()"),
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{testCase.include, &testCase.terragruntOptions, ""}
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
		terragruntOptions options.TerragruntOptions
		expectedOut       string
		expectedErr       error
	}{
		{
			"",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			nil,
		},
		{
			"foo bar",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"foo bar",
			nil,
		},
		{
			"$foo {bar}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"$foo {bar}",
			nil,
		},
		{
			"${path_relative_to_include()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			".",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child",
			nil,
		},
		{
			"${path_relative_to_include()}",
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"child/sub-child",
			nil,
		},
		{
			"foo/${path_relative_to_include()}/bar",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"foo/./bar",
			nil,
		},
		{
			"foo/${path_relative_to_include()}/bar",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"foo/child/bar",
			nil,
		},
		{
			"foo/${path_relative_to_include()}/bar/${path_relative_to_include()}",
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"foo/child/bar/child",
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			"${    find_in_parent_folders()    }",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"../../" + DefaultTerragruntConfigPath,
			nil,
		},
		{
			"${find_in_parent_folders ()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax("${find_in_parent_folders ()}"),
		},
		{
			"foo/${find_in_parent_folders()}/bar",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			fmt.Sprintf("foo/../../%s/bar", DefaultTerragruntConfigPath),
			nil,
		},
		{
			"${find_in_parent_folders()}",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			ParentTerragruntConfigNotFound("../test/fixture-parent-folders/no-terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath),
		},
		{
			"foo/${unknown}/bar",
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax("${unknown}"),
		},
	}

	for _, testCase := range testCases {
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, testCase.include, &testCase.terragruntOptions)
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

	testCases := []struct {
		str               string
		terragruntOptions options.TerragruntOptions
		expectedOut       string
		expectedErr       error
	}{
		{
			"foo/${get_env()}/bar",
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidFunctionParameters(""),
		},
		{
			"foo/${get_env(Invalid Parameters)}/bar",
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax("${get_env(Invalid Parameters)}"),
		},
		{
			"foo/${get_env('env','')}/bar",
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax("${get_env('env','')}"),
		},
		{
			`foo/${get_env("","")}/bar`,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidFunctionParameters(`"",""`),
		},
		{
			`foo/${get_env(   ""    ,   ""    )}/bar`,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidFunctionParameters(`   ""    ,   ""    `),
		},
		{
			`${get_env("SOME_VAR", "SOME{VALUE}")}`,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"SOME{VALUE}",
			nil,
		},
		{
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT","")}/bar`,
			options.TerragruntOptions{
				TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath,
				NonInteractive:       true,
				Env:                  map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"},
			},
			"foo//bar",
			nil,
		},
		{
			`foo/${get_env(    "TEST_ENV_TERRAGRUNT_HIT"   ,   ""   )}/bar`,
			options.TerragruntOptions{
				TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath,
				NonInteractive:       true,
				Env:                  map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"},
			},
			"foo//bar",
			nil,
		},
		{
			`foo/${get_env("TEST_ENV_
TERRAGRUNT_HIT","")}/bar`,
			options.TerragruntOptions{
				TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath,
				NonInteractive:       true,
				Env:                  map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"},
			},
			"",
			InvalidInterpolationSyntax(`${get_env("TEST_ENV_
TERRAGRUNT_HIT","")}`),
		},
		{
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT","DEFAULT")}/bar`,
			options.TerragruntOptions{
				TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath,
				NonInteractive:       true,
				Env:                  map[string]string{"TEST_ENV_TERRAGRUNT_OTHER": "SOMETHING"},
			},
			"foo/DEFAULT/bar",
			nil,
		},
		{
			`foo/${get_env(    "TEST_ENV_TERRAGRUNT_HIT      "   ,   "default"   )}/bar`,
			options.TerragruntOptions{
				TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath,
				NonInteractive:       true,
				Env:                  map[string]string{"TEST_ENV_TERRAGRUNT_HIT": "environment hit  "},
			},
			"foo/environment hit  /bar",
			nil,
		},
		{
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT","default")}/bar`,
			options.TerragruntOptions{
				TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath,
				NonInteractive:       true,
				Env:                  map[string]string{"TEST_ENV_TERRAGRUNT_HIT": "HIT"},
			},
			"foo/HIT/bar",
			nil,
		},
		{
			// Unclosed quote
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT}/bar`,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			"",
			InvalidInterpolationSyntax(`${get_env("TEST_ENV_TERRAGRUNT_HIT}`),
		},
		{
			// Unclosed quote and interpolation pattern
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT/bar`,
			options.TerragruntOptions{TerragruntConfigPath: "/root/child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			`foo/${get_env("TEST_ENV_TERRAGRUNT_HIT/bar`,
			nil,
		},
	}

	for _, testCase := range testCases {
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, mockDefaultInclude, &testCase.terragruntOptions)
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
			util.CommaSeparatedStrings(TERRAFORM_COMMANDS_NEED_LOCKING),
			nil,
		},
		{
			`commands = ["${get_terraform_commands_that_need_vars()}"]`,
			fmt.Sprintf("commands = [%s]", util.CommaSeparatedStrings(TERRAFORM_COMMANDS_NEED_VARS)),
			nil,
		},
		{
			`commands = "test-${get_terraform_commands_that_need_vars()}"`,
			fmt.Sprintf(`commands = "test-%v"`, TERRAFORM_COMMANDS_NEED_VARS),
			nil,
		},
	}

	for _, testCase := range testCases {
		options := options.TerragruntOptions{TerragruntConfigPath: DefaultTerragruntConfigPath, NonInteractive: true}
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, mockDefaultInclude, &options)
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
			fmt.Sprintf(`${get_env("NON_EXISTING_VAR1", "default"-%v`, TERRAFORM_COMMANDS_NEED_VARS),
			nil,
		},
		{
			`test1 = "${get_env("NON_EXISTING_VAR1", "default")}" test2 = ["${get_terraform_commands_that_need_vars()}"]`,
			fmt.Sprintf(`test1 = "default" test2 = [%v]`, util.CommaSeparatedStrings(TERRAFORM_COMMANDS_NEED_VARS)),
			nil,
		},
		{
			`${get_env("NON_EXISTING_VAR1", "default")}-${get_terraform_commands_that_need_vars()}`,
			fmt.Sprintf("default-%v", TERRAFORM_COMMANDS_NEED_VARS),
			nil,
		},
	}

	for _, testCase := range testCases {
		include := mockDefaultInclude
		options := options.TerragruntOptions{TerragruntConfigPath: DefaultTerragruntConfigPath, NonInteractive: true}
		actualOut, actualErr := ResolveTerragruntConfigString(testCase.str, include, &options)
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
	context := resolveContext{mockDefaultInclude, options.NewTerragruntOptionsForTest(configPath), ""}
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
		terragruntOptions options.TerragruntOptions
		expectedPath      string
	}{
		{
			mockDefaultInclude,
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			helpers.RootFolder + "child",
		},
		{
			IncludeConfig{Path: "../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: "../../../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: helpers.RootFolder + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/sub-sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			helpers.RootFolder,
		},
		{
			IncludeConfig{Path: "../../other-child/" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: helpers.RootFolder + "child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			fmt.Sprintf("%s/other-child", filepath.VolumeName(parentDir)),
		},
		{
			IncludeConfig{Path: "../../" + DefaultTerragruntConfigPath},
			options.TerragruntOptions{TerragruntConfigPath: "../child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			parentDir,
		},
		{
			IncludeConfig{Path: "${find_in_parent_folders()}"},
			options.TerragruntOptions{TerragruntConfigPath: "../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/" + DefaultTerragruntConfigPath, NonInteractive: true},
			fmt.Sprintf("%s/test/fixture-parent-folders/terragrunt-in-root", parentDir),
		},
	}

	for _, testCase := range testCases {
		context := resolveContext{testCase.include, &testCase.terragruntOptions, ""}
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

	mockOptions := &options.TerragruntOptions{Variables: options.VariableList{
		"a": {Source: options.Default, Value: "a"},
		"b": {Source: options.Default, Value: "b"},
	}}

	tests := []struct {
		name       string
		args       args
		wantResult []string
		wantErr    bool
	}{
		{"Too much parameters", args{`var.a, var.b, "text", var.c`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, nil, true},
		{"y", args{`var.a, var.b, "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"a", "b", "text"}, false},
		{"Var with -", args{`var.a-1, var.b, "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"", "b", "text"}, false},
		{"With function", args{`var.a-1, default(var.a, "no a"), "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"", "a", "text"}, false},
		{"With default no value", args{`var.a-1, default(var.c, "no c"), "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, []string{"", "no c", "text"}, false},
		{"Wrong number of parameters", args{`var.a-1, default(var.c, "no c", extra), "text"`, regexp.MustCompile(fmt.Sprintf("^%s$", getVarParams(3))), mockOptions}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := resolveContext{mockDefaultInclude, tt.args.terragruntOptions, tt.args.parameters}
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
