package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
)

func TestParseTerragruntOptionsFromArgs(t *testing.T) {
	t.Parallel()

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	workingDir = filepath.ToSlash(workingDir)

	testCases := []struct {
		args            []string
		expectedOptions *options.TerragruntOptions
		expectedErr     error
	}{
		{
			[]string{},
			mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{}, false, "", false),
			nil,
		},

		{
			[]string{"foo", "bar"},
			mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{"foo", "bar"}, false, "", false),
			nil,
		},

		{
			[]string{"--foo", "--bar"},
			mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{"--foo", "--bar"}, false, "", false),
			nil,
		},

		{
			[]string{"--foo", "apply", "--bar"},
			mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{"--foo", "apply", "--bar"}, false, "", false),
			nil,
		},

		{
			[]string{"--terragrunt-non-interactive"},
			mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{}, true, "", false),
			nil,
		},

		{
			[]string{"--terragrunt-config", fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath)},
			mockOptions(fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath), workingDir, []string{}, false, "", false),
			nil,
		},

		{
			[]string{"--terragrunt-working-dir", "/some/path"},
			mockOptions(util.JoinPath("/some/path", config.DefaultTerragruntConfigPath), "/some/path", []string{}, false, "", false),
			nil,
		},

		{
			[]string{"--terragrunt-source", "/some/path"},
			mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{}, false, "/some/path", false),
			nil,
		},

		{
			[]string{"--terragrunt-ignore-dependency-errors"},
			mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{}, false, "", true),
			nil,
		},

		{
			[]string{"--terragrunt-config", fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath), "--terragrunt-non-interactive"},
			mockOptions(fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath), workingDir, []string{}, true, "", false),
			nil,
		},

		{
			[]string{"--foo", "--terragrunt-config", fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath), "bar", "--terragrunt-non-interactive", "--baz", "--terragrunt-working-dir", "/some/path", "--terragrunt-source", "github.com/foo/bar//baz?ref=1.0.3"},
			mockOptions(fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath), "/some/path", []string{"--foo", "bar", "--baz"}, true, "github.com/foo/bar//baz?ref=1.0.3", false),
			nil,
		},

		{
			[]string{"--foo", "--terragrunt-apply-template", "--terragrunt-template-patterns", "test" + string(os.PathListSeparator) + "123"},
			func() *options.TerragruntOptions {
				terragruntOptions := mockOptions(util.JoinPath(workingDir, config.DefaultTerragruntConfigPath), workingDir, []string{"--foo"}, false, "", false)
				terragruntOptions.ApplyTemplate = true
				terragruntOptions.TemplateAdditionalPatterns = []string{"test", "123"}
				return terragruntOptions
			}(),
			nil,
		},

		{
			[]string{"--terragrunt-config"},
			nil,
			ErrArgMissingValue("terragrunt-config"),
		},

		{
			[]string{"--terragrunt-working-dir"},
			nil,
			ErrArgMissingValue("terragrunt-working-dir"),
		},

		{
			[]string{"--foo", "bar", "--terragrunt-config"},
			nil,
			ErrArgMissingValue("terragrunt-config"),
		},
	}

	for _, testCase := range testCases {
		actualOptions, actualErr := parseTerragruntOptionsFromArgs(testCase.args)
		if testCase.expectedErr != nil {
			assert.True(t, errors.IsError(actualErr, testCase.expectedErr), "Expected error %v but got error %v", testCase.expectedErr, actualErr)
		} else {
			assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
			assertOptionsEqual(t, *testCase.expectedOptions, *actualOptions, "For args %v", testCase.args)
		}
	}
}

// We can't do a direct comparison between TerragruntOptions objects because we can't compare Logger or RunTerragrunt
// instances. Therefore, we have to manually check everything else.
func assertOptionsEqual(t *testing.T, expected options.TerragruntOptions, actual options.TerragruntOptions, msgAndArgs ...interface{}) {
	assert.NotNil(t, expected.Logger, msgAndArgs...)
	assert.NotNil(t, actual.Logger, msgAndArgs...)

	assert.Equal(t, expected.TerragruntConfigPath, actual.TerragruntConfigPath, msgAndArgs...)
	assert.Equal(t, expected.NonInteractive, actual.NonInteractive, msgAndArgs...)
	assert.Equal(t, expected.TerraformCliArgs, actual.TerraformCliArgs, msgAndArgs...)
	assert.Equal(t, expected.WorkingDir, actual.WorkingDir, msgAndArgs...)
	assert.Equal(t, expected.Source, actual.Source, msgAndArgs...)
	assert.Equal(t, expected.IgnoreDependencyErrors, actual.IgnoreDependencyErrors, msgAndArgs...)
	assert.Equal(t, expected.ApplyTemplate, actual.ApplyTemplate, msgAndArgs...)
	assert.Equal(t, expected.TemplateAdditionalPatterns, actual.TemplateAdditionalPatterns, msgAndArgs...)
	assert.Equal(t, expected.BootConfigurationPaths, actual.BootConfigurationPaths, msgAndArgs...)
}

func mockOptions(terragruntConfigPath string, workingDir string, terraformCliArgs []string, nonInteractive bool, terragruntSource string, ignoreDependencyErrors bool) *options.TerragruntOptions {
	opts := options.NewTerragruntOptionsForTest(terragruntConfigPath)

	opts.WorkingDir = workingDir
	opts.TerraformCliArgs = terraformCliArgs
	opts.NonInteractive = nonInteractive
	opts.Source = terragruntSource
	opts.IgnoreDependencyErrors = ignoreDependencyErrors

	return opts
}

func TestFilterTerragruntArgs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		args     []string
		expected []string
	}{
		{[]string{}, []string{}},
		{[]string{"foo", "--bar"}, []string{"foo", "--bar"}},
		{[]string{"foo", "--terragrunt-config", fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath)}, []string{"foo"}},
		{[]string{"foo", "--terragrunt-non-interactive"}, []string{"foo"}},
		{[]string{"foo", "--terragrunt-non-interactive", "--bar", "--terragrunt-working-dir", "/some/path", "--baz", "--terragrunt-config", fmt.Sprintf("/some/path/%s", config.DefaultTerragruntConfigPath)}, []string{"foo", "--bar", "--baz"}},
		{[]string{"apply-all", "foo", "bar"}, []string{"foo", "bar"}},
		{[]string{"foo", "destroy-all", "--foo", "--bar"}, []string{"foo", "--foo", "--bar"}},
	}

	for _, testCase := range testCases {
		actual := filterTerragruntArgs(testCase.args)
		assert.Equal(t, testCase.expected, actual, "For args %v", testCase.args)
	}
}

func TestParseEnvironmentVariables(t *testing.T) {
	testCases := []struct {
		environmentVariables []string
		expectedVariables    map[string]string
	}{
		{
			[]string{},
			map[string]string{},
		},
		{
			[]string{"foobar"},
			map[string]string{},
		},
		{
			[]string{"foo=bar"},
			map[string]string{"foo": "bar"},
		},
		{
			[]string{"foo=bar", "goo=gar"},
			map[string]string{"foo": "bar", "goo": "gar"},
		},
		{
			[]string{"foo=bar   "},
			map[string]string{"foo": "bar   "},
		},
		{
			[]string{"foo   =bar   "},
			map[string]string{"foo": "bar   "},
		},
		{
			[]string{"foo=composite=bar"},
			map[string]string{"foo": "composite=bar"},
		},
		{
			[]string{"TF_VAR_foo=composite=bar"},
			map[string]string{"TF_VAR_foo": "composite=bar"},
		},
	}

	for _, testCase := range testCases {
		var mockOptions = options.NewTerragruntOptionsForTest("test-env-mock")
		parseEnvironmentVariables(mockOptions, testCase.environmentVariables)
		assert.Equal(t, testCase.expectedVariables, mockOptions.Env)
	}
}

func Test_filterVarsAndVarFiles(t *testing.T) {
	type ov = options.Variable
	type variables = map[string]ov
	param, varfile := options.VarParameterExplicit, options.VarFileExplicit

	tests := []struct {
		name    string
		command string
		args    []string
		want    []string // nil could be used to specify unaltered list
		vars    variables
		wantErr bool
	}{
		{"No args", "plan", []string{}, nil, variables{}, false},
		{"-var", "plan", []string{"-var", "foo=bar"}, nil, variables{
			"foo": ov{Source: param, Value: "bar"},
		}, false},
		{"--var", "plan", []string{"--var", "foo=bar"}, nil, variables{
			"foo": ov{Source: param, Value: "bar"},
		}, false},
		{"With value", "plan", []string{"--var=foo=bar", "-var=bar=foo"}, nil, variables{
			"foo": ov{Source: param, Value: "bar"},
			"bar": ov{Source: param, Value: "foo"},
		}, false},
		{"Separated", "plan", []string{"-var", "foo=bar"}, nil, variables{
			"foo": ov{Source: param, Value: "bar"},
		}, false},
		{"With empty value", "plan", []string{"--var", "foo="}, nil, variables{
			"foo": ov{Source: param, Value: ""},
		}, false},
		{"With non real arg", "plan", []string{"-var", "foo=bar", "---var=test"}, nil, variables{
			"foo": ov{Source: param, Value: "bar"},
		}, false},
		{"With invalid value", "plan", []string{"--var=foo"}, nil, variables{}, true},
		{"With invalid value 2", "plan", []string{"--var="}, nil, variables{}, true},
		{"With invalid value 3", "plan", []string{"--var"}, nil, variables{}, true},
		{"With invalid value 4", "plan", []string{"--var", ""}, nil, variables{}, true},
		{"With invalid value 5", "plan", []string{"-var", "foo"}, nil, variables{}, true},
		{"With invalid value 6", "plan", []string{"-var-file"}, nil, variables{}, true},
		{"With invalid value 7", "plan", []string{"-var-file="}, nil, variables{}, true},
		{"With invalid file", "plan", []string{"-var-file", "foo"}, nil, variables{}, true},
		{"With var and var-file", "plan", []string{"-var", "bar=foo", "-var-file", "../test/fixture-args/test.tfvars"}, nil, variables{
			"foo": ov{Source: varfile, Value: "bar"},
			"int": ov{Source: varfile, Value: 1},
			"bar": ov{Source: param, Value: "foo"},
		}, false},
		{"With var overwrite", "plan", []string{"-var", "foo=overwritten", "-var-file", "../test/fixture-args/test.tfvars"}, nil, variables{
			"foo": ov{Source: param, Value: "overwritten"},
			"int": ov{Source: varfile, Value: 1},
		}, false},
		{"With filtered arguments", "whatever",
			[]string{"-test", "dummy", "-var", "foo=yes", "other", "-var-file", "../test/fixture-args/test.tfvars"},
			[]string{"-test", "dummy", "other"},
			variables{
				"foo": ov{Source: param, Value: "yes"},
				"int": ov{Source: varfile, Value: 1},
			}, false},
	}
	for _, tt := range tests {
		var mockOptions = options.NewTerragruntOptionsForTest("test-env-mock")
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterVarsAndVarFiles(tt.command, mockOptions, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterVarsAndVarFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.want == nil {
				tt.want = tt.args
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterVarsAndVarFiles() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(mockOptions.Variables, tt.vars) {
				t.Errorf("variables = %v, want %v", mockOptions.Variables, tt.vars)
			}
		})
	}
}

func Test_convertToNativeType(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want interface{}
	}{
		{"Empty", "", ""},
		{"Spaces", "   ", "   "},
		{"Zero", "0", 0},
		{"Positive", "1234", 1234},
		{"Negative", "-1234", -1234},
		{"Float", "12.34", 12.34},
		{"Negative Float", "-12.34", -12.34},
		{"Bool true", "true", true},
		{"Bool false", "false", false},
		{"Bool f", "f", false},
		{"Bool T", "T", true},
		{"With spaces", "  1234 ", 1234},
		{"Exp1", "2e10", 2e10},
		{"Exp2", "2e-10", 2e-10},
		{"Exp3", "-2e-10", -2e-10},
		{"String", "Anything else", "Anything else"},
		{"Quoted string", "'1234'", "1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToNativeType(tt.s)
			assert.EqualValues(t, tt.want, got)
		})
	}
}
