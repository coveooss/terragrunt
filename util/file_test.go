package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coveooss/terragrunt/v2/test/helpers"
	"github.com/stretchr/testify/assert"
)

func TestGetPathRelativeTo(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		path     string
		basePath string
		expected string
	}{
		{"", "", "."},
		{helpers.RootFolder, helpers.RootFolder, "."},
		{helpers.RootFolder, helpers.RootFolder + "child", ".."},
		{helpers.RootFolder, helpers.RootFolder + "child/sub-child/sub-sub-child", "../../.."},
		{helpers.RootFolder + "other-child", helpers.RootFolder + "child", "../other-child"},
		{helpers.RootFolder + "other-child/sub-child", helpers.RootFolder + "child/sub-child", "../../other-child/sub-child"},
		{helpers.RootFolder + "root", helpers.RootFolder + "other-root", "../root"},
		{helpers.RootFolder + "root", helpers.RootFolder + "other-root/sub-child/sub-sub-child", "../../../root"},
	}

	for _, testCase := range testCases {
		actual, err := GetPathRelativeTo(testCase.path, testCase.basePath)
		assert.Nil(t, err, "Unexpected error for path %s and basePath %s: %v", testCase.path, testCase.basePath, err)
		assert.Equal(t, testCase.expected, actual, "For path %s and basePath %s", testCase.path, testCase.basePath)
	}
}

func TestCanonicalPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		path     string
		basePath string
		expected string
	}{
		{"", helpers.RootFolder + "foo", helpers.RootFolder + "foo"},
		{".", helpers.RootFolder + "foo", helpers.RootFolder + "foo"},
		{"bar", helpers.RootFolder + "foo", helpers.RootFolder + "foo/bar"},
		{"bar/baz/blah", helpers.RootFolder + "foo", helpers.RootFolder + "foo/bar/baz/blah"},
		{"bar/../blah", helpers.RootFolder + "foo", helpers.RootFolder + "foo/blah"},
		{"bar/../..", helpers.RootFolder + "foo", helpers.RootFolder},
		{"bar/.././../baz", helpers.RootFolder + "foo", helpers.RootFolder + "baz"},
		{"bar", helpers.RootFolder + "foo/../baz", helpers.RootFolder + "baz/bar"},
		{"a/b/../c/d/..", helpers.RootFolder + "foo/../baz/.", helpers.RootFolder + "baz/a/c"},
		{helpers.RootFolder + "other", helpers.RootFolder + "foo", helpers.RootFolder + "other"},
		{helpers.RootFolder + "other/bar/blah", helpers.RootFolder + "foo", helpers.RootFolder + "other/bar/blah"},
		{helpers.RootFolder + "other/../blah", helpers.RootFolder + "foo", helpers.RootFolder + "blah"},
	}

	for _, testCase := range testCases {
		actual, err := CanonicalPath(testCase.path, testCase.basePath)
		assert.Nil(t, err, "Unexpected error for path %s and basePath %s: %v", testCase.path, testCase.basePath, err)
		assert.Equal(t, testCase.expected, actual, "For path %s and basePath %s", testCase.path, testCase.basePath)
	}
}

func TestPathContainsHiddenFileOrFolder(t *testing.T) {
	testCases := []struct {
		path     string
		expected bool
	}{
		{"", false},
		{".", false},
		{".foo", true},
		{".foo/", true},
		{"foo/bar", false},
		{"/foo/bar", false},
		{".foo/bar", true},
		{"foo/.bar", true},
		{"/foo/.bar", true},
		{"/foo/./bar", false},
		{"/foo/../bar", false},
		{"/foo/.././bar", false},
		{"/foo/.././.bar", true},
		{"/foo/.././.bar/", true},
	}

	for _, testCase := range testCases {
		path := filepath.FromSlash(testCase.path)
		actual := PathContainsHiddenFileOrFolder(path)
		assert.Equal(t, testCase.expected, actual, "For path %s", path)
	}
}

func TestExpandArgs(t *testing.T) {
	testCases := []struct {
		args     []interface{}
		expected []interface{}
	}{
		{[]interface{}{"1"}, []interface{}{"1"}},
		{[]interface{}{"something.something\\[0\\]"}, []interface{}{"something.something\\[0\\]"}},
		{[]interface{}{"-target"}, []interface{}{"-target"}},
		{[]interface{}{"-lock-timeout=20m"}, []interface{}{"-lock-timeout=20m"}},
		{[]interface{}{"wordsWithBrackets[]"}, []interface{}{"wordsWithBrackets[]"}},
		{[]interface{}{"envExpansion", "$USER"}, []interface{}{"envExpansion", os.Getenv("USER")}},
		{[]interface{}{"-going-to-expand", "../test/fixture-expand/*"}, []interface{}{"-going-to-expand", "../test/fixture-expand/1", "../test/fixture-expand/2", "../test/fixture-expand/3", "../test/fixture-expand/4"}},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.expected, ExpandArguments(tc.args, "."))
	}
}
