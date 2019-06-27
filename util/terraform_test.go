package util

import (
	"path/filepath"
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
	}{
		{"All Types", args{testFixtureDefaultValues}, map[string]interface{}{
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
		}},
		{"Invalid Folder", args{"Invalid"}, map[string]interface{}{}},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := LoadDefaultValues(tt.args.folder)
			assert.Nil(t, err)
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("LoadDefaultValues():\ngot : %v\nwant: %v", collections.AsDictionary(gotResult), collections.AsDictionary(tt.wantResult))
			}
		})
	}
}

func Test_getTerraformFiles(t *testing.T) {
	resultFiles := []string{
		"a.tf", "a.tf.json",
		"b.tf", "b.tf.json",
		"c.tf",
		"a_override.tf",
		"b_override.tf.json",
		"override.tf", "override.tf.json",
		"z_override.tf",
	}

	expectedResult := make([]string, 0, len(resultFiles))
	for _, value := range resultFiles {
		expectedResult = append(expectedResult, filepath.Join(testFixtureDefaultValues, value))
	}

	type args struct {
		folder string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"All Types", args{testFixtureDefaultValues}, expectedResult},
		{"Invalid Folder", args{"Invalid"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTerraformFiles(tt.args.folder)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTerraformFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}
