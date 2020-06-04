package config

import (
	"testing"

	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/test/helpers"
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
		if context.include.Path == "${find_in_parent_folders()}" {
			path, _ := context.findInParentFolders()
			context.include.Path = path
		}
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
		if context.include.Path == "${find_in_parent_folders()}" {
			path, _ := context.findInParentFolders()
			context.include.Path = path
		}
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
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "For options %v, expected error %v but got error %v\nResult = %v", testCase.terragruntOptions, testCase.expectedErr, actualErr, actualPath)
		} else {
			assert.Nil(t, actualErr, "For options %v, unexpected error: %v", testCase.terragruntOptions, actualErr)
			assert.Equal(t, testCase.expectedPath, actualPath, "For options %v", testCase.terragruntOptions)
		}
	}
}
