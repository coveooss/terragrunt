package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testFixtureDefaultValues = "../test/fixture-default-values/variables-files"
)

func TestLoadDefaultValues(t *testing.T) {
	testCases := []struct {
		name       string
		folder     string
		wantResult map[string]interface{}
		err        string
	}{
		{"All Types",
			testFixtureDefaultValues,
			map[string]interface{}{
				"a":               "A (a.tf)",
				"a_overridden_1":  "A (a_override.tf)",
				"a_overridden_2":  "A (override.tf)",
				"aj":              "AJ (a.tf.json)",
				"aj_overridden_1": "AJ (override.tf.json)",
				"b":               "B (b.tf)",
				"b_overridden_1":  "B (override.tf)",
				"b_overridden_2":  "B (z_override.tf)",
				"bj":              "BJ (b.tf.json)",
				"bj_overridden_1": "BJ (b_override.tf.json)",
				"bj_overridden_2": "BJ (override.tf.json)",
			},
			"",
		},
		{"Invalid Folder", "Invalid", nil, "Module directory Invalid does not exist or cannot be read"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, _, err := LoadDefaultValues(tt.folder, nil, true)
			assert.Equal(t, tt.wantResult, gotResult)
			if tt.err != "" {
				assert.Error(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
