package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunConditions(t *testing.T) {
	t.Parallel()

	type vars = map[string]interface{}
	tests := []struct {
		name      string
		config    string
		variables vars
		expected  bool
	}{
		{
			name: "Simple match",
			config: `
				run_conditions {
					run_if = {
						env = "qa"
					}
				}
			`,
			variables: vars{"env": "qa"},
			expected:  true,
		},
		{
			name: "Simple match (map)",
			config: `
				run_conditions {
					run_if = {
						"my_map.env" = "qa"
					}
				}
			`,
			variables: vars{"my_map": map[string]interface{}{"env": "qa"}},
			expected:  true,
		},
		{
			name: "Simple no match",
			config: `
				run_conditions {
					run_if = {
						"env" = "qa"
					}
				}
			`,
			variables: vars{"env": "dev"},
			expected:  false,
		},
		{
			name: "Simple no match (map)",
			config: `
				run_conditions {
					run_if = {
						"my_map.env" = "qa"
					}
				}
			`,
			variables: vars{"my_map": map[string]interface{}{"env": "dev"}},
			expected:  false,
		},
		{
			name: "Two possibilities",
			config: `
				run_conditions {
					run_if = {
						"env" = ["dev", "qa"]
					}
				}
			`,
			variables: vars{"env": "qa"},
			expected:  true,
		},
		{
			name: "Two conditions (no match)",
			config: `
				run_conditions {
					run_if = {
						"env" = ["dev", "qa"]
						"region" = "us-east-1"
					}
				}
			`,
			variables: vars{"env": "qa", "region": "us-west-2"},
			expected:  false,
		},
		{
			name: "Ignore simple",
			config: `
				run_conditions {
					ignore_if = {
						"env" = "qa"
					}
				}
			`,
			variables: vars{"env": "qa"},
			expected:  false,
		},
		{
			name: "Ignore simple (map)",
			config: `
				run_conditions {
					ignore_if = {
						"my_map.env" = "qa"
					}
				}
			`,
			variables: vars{"my_map": map[string]interface{}{"env": "qa"}},
			expected:  false,
		},
		{
			name: "Ignore simple: not ignored",
			config: `
				run_conditions {
					ignore_if = {
						"env" = "qa"
					}
				}
			`,
			variables: vars{"env": "dev"},
			expected:  true,
		},
		{
			name: "Ignore two possibilities: ignored",
			config: `
				run_conditions {
					ignore_if = {
						"env" = ["dev", "qa"]
					}
				}
			`,
			variables: vars{"env": "qa"},
			expected:  false,
		},
		{
			name: "Run & Ignore two condition: partial match",
			config: `
				run_conditions {
					run_if = {
						"env" = ["dev", "qa"]
					}
				}

				run_conditions {
					ignore_if = {
						"region" = "us-west-2"
					}
				}
			`,
			variables: vars{"env": "qa", "region": "us-west-2"},
			expected:  false,
		},
		{
			name: "Run & Ignore two condition: partial match (two conditions in the same run_conditions)",
			config: `
				run_conditions {
					run_if = {
						"env" = ["dev", "qa"]
					}
					ignore_if = {
						"region" = "us-west-2"
					}
				}
			`,
			variables: vars{"env": "qa", "region": "us-west-2"},
			expected:  false,
		},
		{
			name: "Run & Ignore two condition: not ignored",
			config: `
				run_conditions {
					run_if = {
						"env" = ["dev", "qa"]
					}
				}

				run_conditions {
					ignore_if = {
						"region" = "us-west-2"
					}
				}
			`,
			variables: vars{"env": "qa", "region": "us-east-1"},
			expected:  true,
		},
		{
			name: "Run & Ignore two condition: not ignored (two conditions in the same run_conditions)",
			config: `
				run_conditions {
					run_if = {
						"env" = ["dev", "qa"]
					}

					ignore_if = {
						"region" = "us-west-2"
					}
				}
			`,
			variables: vars{"env": "qa", "region": "us-east-1"},
			expected:  true,
		},
		{
			name: "Run & Ignore two conditions: no match",
			config: `
				run_conditions {
					run_if = {
						"env" = ["dev", "qa"]
					}
				}

				run_conditions {
					ignore_if = {
						"region" = "us-west-2"
					}
				}
			`,
			variables: vars{"env": "prod", "region": "us-east-1"},
			expected:  false,
		},
		{
			name: "Key as composite variables",
			config: `
			run_conditions {
				run_if = {
					"${var.env}/${var.region}" = ["qa/us-west2-2", "dev/us-east-1"]
				}
			}

			run_conditions {
				ignore_if = {
					"region" = "us-west-2"
				}
			}
			`,
			variables: vars{"env": "dev", "region": "us-east-1"},
			expected:  true,
		},
		{
			name: "Deny with key as composite variables",
			config: `
			run_conditions {
				ignore_if = {
					"${var.env}/${var.region}" = ["qa/us-west2-2", "dev/us-east-1"]
				}
			}

			run_conditions {
				ignore_if = {
					"region"                   = "us-west-2"
				}
			}
			`,
			variables: vars{"env": "dev", "region": "us-east-1"},
			expected:  false,
		},
		{
			name: "Invalid variable on ignore",
			config: `
			run_conditions {
				ignore_if = {
					"non_existing" = "value"
				}
			}
			`,
			variables: vars{},
			expected:  false,
		},
		{
			name: "Invalid variables on ignore (map)",
			config: `
			run_conditions {
				ignore_if = {
					"my_map.envv" = "value"
				}
			}
			`,
			variables: vars{"my_map": map[string]interface{}{"env": "qa"}},
			expected:  false,
		},
		{
			name: "Invalid variables on run_if",
			config: `
			run_conditions {
				run_if = {
					"non_existing" = "value"
				}
			}
			`,
			variables: vars{},
			expected:  false,
		},
		{
			name: "Invalid variables on run_if (map)",
			config: `
			run_conditions {
				run_if = {
					"my_map.envv" = "value"
				}
			}
			`,
			variables: vars{"my_map": map[string]interface{}{"env": "qa"}},
			expected:  false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := newOptionsVariables(tt.variables)
			config, err := parseConfigString(tt.config, opts, IncludeConfig{Path: opts.TerragruntConfigPath})
			if err != nil {
				assert.Fail(t, fmt.Sprintf("Caught error while parsing config: %v", err))
			} else {
				assert.Equal(t, tt.expected, config.RunConditions.ShouldRun())
			}
		})
	}
}
