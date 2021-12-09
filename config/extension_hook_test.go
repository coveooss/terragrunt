package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHookTypeStringConversion(t *testing.T) {
	testCases := []struct {
		name           string
		hookType       HookType
		expectedString string
	}{
		{name: "UnsetHookType", hookType: UnsetHookType, expectedString: "(unset hook type!)"},
		{name: "PreHookType", hookType: PreHookType, expectedString: "pre_hook"},
		{name: "PostHookType", hookType: PostHookType, expectedString: "post_hook"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := testCase.hookType.String()
			assert.Equal(t, testCase.expectedString, actual)
		})
	}
}
