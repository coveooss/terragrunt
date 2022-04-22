package configstack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSummaryResultFromPlan(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		planOutput string
		expected   planSummary
	}{
		{
			name:       "no changes with terraform@0.12",
			planOutput: "stuff No changes. Infrastructure is up-to-date. stuff",
			expected:   planSummary{"No change", 0, true},
		},
		{
			name:       "no changes with terraform@0.13",
			planOutput: "stuff Plan: 0 to add, 0 to change, 0 to destroy. stuff",
			expected:   planSummary{"No change", 0, true},
		},
		{
			name:       "no changes with terraform@1.0.2",
			planOutput: "stuff Your infrastructure matches the configuration. stuff",
			expected:   planSummary{"No change", 0, true},
		},
		{
			name:       "with changes",
			planOutput: "stuff 11 to add, 10 to change, 21 to destroy. stuff",
			expected:   planSummary{"11 to add, 10 to change, 21 to destroy.", 42, true},
		},
		{
			name:       "unknown number of changes",
			planOutput: "Nobody knows ~~ 👻 ~~ Spooky",
			expected:   planSummary{"Unable to determine the plan status", -1, false},
		},
		{
			name:       "no changes without a plan",
			planOutput: "stuff SomethingElse: 0 to add, 0 to change, 0 to destroy. stuff",
			expected:   planSummary{"No effective change", 0, true},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			summary := extractSummaryResultFromPlan(testCase.planOutput)
			assert.Equal(t, testCase.expected, summary)
		})
	}
}
