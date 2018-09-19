package config

import (
	"fmt"
	"testing"

	"github.com/coveo/gotemplate/hcl"
	"github.com/stretchr/testify/assert"
)

func TestRunConditions(t *testing.T) {
	t.Parallel()

	type list = hcl.List
	type vars = map[string]interface{}
	tests := []struct {
		name      string
		allowed   []Condition
		denied    []Condition
		variables vars
		expected  bool
	}{
		{
			name:      "Simple match",
			allowed:   []Condition{{"env": "qa"}},
			variables: vars{"env": "qa"},
			expected:  true,
		},
		{
			name:      "Simple no match",
			allowed:   []Condition{{"env": "qa"}},
			variables: vars{"env": "dev"},
			expected:  false,
		},
		{
			name:      "Two possibilities",
			allowed:   []Condition{{"env": list{"dev", "qa"}}},
			variables: vars{"env": "qa"},
			expected:  true,
		},
		{
			name:      "Two conditions (no match)",
			allowed:   []Condition{{"env": list{"dev", "qa"}, "region": "us-east-1"}},
			variables: vars{"env": "qa", "region": "us-west-2"},
			expected:  false,
		},
		{
			name:      "Ignore simple",
			denied:    []Condition{{"env": "qa"}},
			variables: vars{"env": "qa"},
			expected:  false,
		},
		{
			name:      "Ignore simple: not ignored",
			denied:    []Condition{{"env": "qa"}},
			variables: vars{"env": "dev"},
			expected:  true,
		},
		{
			name:      "Ignore two possibilities: ignored",
			denied:    []Condition{{"env": list{"dev", "qa"}}},
			variables: vars{"env": "qa"},
			expected:  false,
		},
		{
			name:      "Run & Ignore two condition: partial match",
			allowed:   []Condition{{"env": list{"dev", "qa"}}},
			denied:    []Condition{{"region": "us-west-2"}},
			variables: vars{"env": "qa", "region": "us-west-2"},
			expected:  false,
		},
		{
			name:      "Run & Ignore two condition: not ignored",
			allowed:   []Condition{{"env": list{"dev", "qa"}}},
			denied:    []Condition{{"region": "us-west-2"}},
			variables: vars{"env": "qa", "region": "us-east-1"},
			expected:  true,
		},
		{
			name:      "Run & Ignore two conditions: no match",
			allowed:   []Condition{{"env": list{"dev", "qa"}}},
			denied:    []Condition{{"region": "us-west-2"}},
			variables: vars{"env": "prod", "region": "us-east-1"},
			expected:  false,
		},
		{
			name:      "Key as composite variables",
			allowed:   []Condition{{"${var.env}/${var.region}": list{"qa/us-west2-2", "dev/us-east-1"}}},
			denied:    []Condition{{"region": "us-west-2"}},
			variables: vars{"env": "dev", "region": "us-east-1"},
			expected:  true,
		},
		{
			name:      "Deny with key as composite variables",
			denied:    []Condition{{"${var.env}/${var.region}": list{"qa/us-west-2", "dev/us-east-1"}}, {"region": "us-west-2"}},
			variables: vars{"env": "dev", "region": "us-east-1"},
			expected:  false,
		},
		{
			name:      "Deny with key as composite variables",
			denied:    []Condition{{"${var.env}/${var.region}": list{"qa/us-west-2", "dev/us-east-1"}}, {"region": "us-west-2"}},
			variables: vars{"env": "dev", "region": "us-east-1"},
			expected:  false,
		},
		{
			name:      "Invalid variable on ignore",
			denied:    []Condition{{"non_existing": "value"}},
			variables: vars{},
			expected:  false,
		},
		{
			name:      "Invalid variables on run_if",
			allowed:   []Condition{{"non_existing": "value"}},
			variables: vars{},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := RunConditions{Allows: tt.allowed, Denies: tt.denied}
			options := newOptionsVariables(tt.variables)
			for _, c := range conditions.Allows {
				c.init(options)
			}
			for _, c := range conditions.Denies {
				c.init(options)
			}
			_ = fmt.Sprint(conditions)
			assert.Equal(t, tt.expected, conditions.ShouldRun())
		})
	}
}
