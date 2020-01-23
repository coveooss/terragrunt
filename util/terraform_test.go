package util

import (
	"reflect"
	"testing"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/stretchr/testify/assert"
)

const (
	testFixtureDefaultValues = "../test/fixture-default-values/variables-files"
)

func TestLoadDefaultValues(t *testing.T) {
	type args struct {
		folder string
	}
	testCases := []struct {
		name       string
		args       args
		wantResult map[string]interface{}
		wantErr    bool
	}{
		{"All Types",
			args{testFixtureDefaultValues},
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
			false,
		},
		{
			"Invalid Folder",
			args{"Invalid"},
			map[string]interface{}{},
			true,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, _, err := LoadDefaultValues(tt.args.folder)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("LoadDefaultValues():\ngot : %v\nwant: %v", collections.AsDictionary(gotResult), collections.AsDictionary(tt.wantResult))
			}
		})
	}
}
