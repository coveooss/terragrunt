package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListContainsElement(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		list     []string
		element  string
		expected bool
	}{
		{[]string{}, "", false},
		{[]string{}, "foo", false},
		{[]string{"foo"}, "foo", true},
		{[]string{"bar", "foo", "baz"}, "foo", true},
		{[]string{"bar", "foo", "baz"}, "nope", false},
		{[]string{"bar", "foo", "baz"}, "", false},
	}

	for _, testCase := range testCases {
		actual := ListContainsElement(testCase.list, testCase.element)
		assert.Equal(t, testCase.expected, actual, "For list %v and element %s", testCase.list, testCase.element)
	}
}

func TestListContainsElementInterface(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		list     []interface{}
		element  interface{}
		expected bool
	}{
		{[]interface{}{}, "", false},
		{[]interface{}{}, 1, false},
		{[]interface{}{"foo"}, "foo", true},
		{[]interface{}{"bar", 2, "baz"}, 2, true},
		{[]interface{}{"bar", "foo", 4}, "nope", false},
		{[]interface{}{"bar", "foo", "baz"}, "", false},
	}

	for _, testCase := range testCases {
		actual := ListContainsElementInterface(testCase.list, testCase.element)
		assert.Equal(t, testCase.expected, actual, "For list %v and element %s", testCase.list, testCase.element)
	}
}

func TestRemoveElementFromList(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		list     []string
		element  string
		expected []string
	}{
		{[]string{}, "", []string{}},
		{[]string{}, "foo", []string{}},
		{[]string{"foo"}, "foo", []string{}},
		{[]string{"bar"}, "foo", []string{"bar"}},
		{[]string{"bar", "foo", "baz"}, "foo", []string{"bar", "baz"}},
		{[]string{"bar", "foo", "baz"}, "nope", []string{"bar", "foo", "baz"}},
		{[]string{"bar", "foo", "baz"}, "", []string{"bar", "foo", "baz"}},
	}

	for _, testCase := range testCases {
		actual := RemoveElementFromList(testCase.list, testCase.element)
		assert.Equal(t, testCase.expected, actual, "For list %v and element %s", testCase.list, testCase.element)
	}
}

func TestRemoveDuplicatesFromList(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		list     []string
		expected []string
		reverse  bool
	}{
		{[]string{}, []string{}, false},
		{[]string{"foo"}, []string{"foo"}, false},
		{[]string{"foo", "bar"}, []string{"foo", "bar"}, false},
		{[]string{"foo", "bar", "foobar", "bar", "foo"}, []string{"foo", "bar", "foobar"}, false},
		{[]string{"foo", "bar", "foobar", "foo", "bar"}, []string{"foo", "bar", "foobar"}, false},
		{[]string{"foo", "bar", "foobar", "bar", "foo"}, []string{"foobar", "bar", "foo"}, true},
		{[]string{"foo", "bar", "foobar", "foo", "bar"}, []string{"foobar", "foo", "bar"}, true},
	}

	for _, testCase := range testCases {
		f := RemoveDuplicatesFromListKeepFirst
		if testCase.reverse {
			f = RemoveDuplicatesFromListKeepLast
		}
		assert.Equal(t, f(testCase.list), testCase.expected, "For list %v", testCase.list)
		t.Logf("%v passed", testCase.list)
	}
}

func TestCommaSeparatedStrings(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		list     []string
		expected string
	}{
		{[]string{}, ``},
		{[]string{"foo"}, `"foo"`},
		{[]string{"foo", "bar"}, `"foo", "bar"`},
	}

	for _, testCase := range testCases {
		assert.Equal(t, CommaSeparatedStrings(testCase.list), testCase.expected, "For list %v", testCase.list)
		t.Logf("%v passed", testCase.list)
	}
}
