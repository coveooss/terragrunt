package util

import (
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKindOf(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		value    interface{}
		expected reflect.Kind
	}{
		{1, reflect.Int},
		{2.0, reflect.Float64},
		{'A', reflect.Int32},
		{math.Pi, reflect.Float64},
		{true, reflect.Bool},
		{nil, reflect.Invalid},
		{"Hello World!", reflect.String},
		{new(string), reflect.Ptr},
		{*new(string), reflect.String},
		{interface{}(false), reflect.Bool},
	}
	for _, testCase := range testCases {
		actual := KindOf(testCase.value).String()
		assert.Equal(t, testCase.expected.String(), actual, "For value %v", testCase.value)
		t.Logf("%v passed", testCase.value)
	}
}

func TestConvertToMap(t *testing.T) {
	type args struct {
		value interface{}
		keys  []string
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		{"Simple value", args{1, nil}, 1},
		{"Simple value", args{1, []string{"a"}}, map[string]interface{}{"a": 1}},
		{"Simple value", args{1, []string{"a", "b"}}, map[string]interface{}{"a": map[string]interface{}{"b": 1}}},
		{"Simple value", args{1, []string{"a", "b", "c"}}, map[string]interface{}{"a": map[string]interface{}{"b": map[string]interface{}{"c": 1}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertToMap(tt.args.value, tt.args.keys...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
