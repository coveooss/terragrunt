package configstack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/errors"
	"github.com/stretchr/testify/assert"
)

func TestResolveTerraformModulesNoPaths(t *testing.T) {
	t.Parallel()

	configPaths := []string{}
	expected := []*TerraformModule{}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesOneModuleNoDependencies(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              canonical(t, "../test/fixture-modules/module-a"),
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{Terraform: &config.TerraformConfig{Source: "test"}},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-a/"+config.DefaultConfigName)),
	}

	configPaths := []string{"../test/fixture-modules/module-a/" + config.DefaultConfigName}
	expected := []*TerraformModule{moduleA}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesOneModuleWithIncludesNoDependencies(t *testing.T) {
	t.Parallel()

	moduleB := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-b/module-b-child"),
		Dependencies: []*TerraformModule{},
		Config: config.TerragruntConfig{
			RemoteState: state(t, "bucket", "module-b-child/terraform.tfstate"),
			Terraform:   &config.TerraformConfig{Source: "..."},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-b/module-b-child/"+config.DefaultConfigName)),
	}

	configPaths := []string{"../test/fixture-modules/module-b/module-b-child/" + config.DefaultConfigName}
	expected := []*TerraformModule{moduleB}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesTwoModulesWithDependencies(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-a"),
		Dependencies: []*TerraformModule{},
		Config: config.TerragruntConfig{
			Terraform: &config.TerraformConfig{Source: "test"},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-a/"+config.DefaultConfigName)),
	}

	moduleC := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-c"),
		Dependencies: []*TerraformModule{moduleA},
		Config: config.TerragruntConfig{
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-a")}},
			Terraform:    &config.TerraformConfig{Source: "temp"},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-c/"+config.DefaultConfigName)),
	}

	configPaths := []string{"../test/fixture-modules/module-a/" + config.DefaultConfigName, "../test/fixture-modules/module-c/" + config.DefaultConfigName}
	expected := []*TerraformModule{moduleA, moduleC}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesMultipleModulesWithDependencies(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              canonical(t, "../test/fixture-modules/module-a"),
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{Terraform: &config.TerraformConfig{Source: "test"}},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-a/"+config.DefaultConfigName)),
	}

	moduleB := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-b/module-b-child"),
		Dependencies: []*TerraformModule{},
		Config: config.TerragruntConfig{
			RemoteState: state(t, "bucket", "module-b-child/terraform.tfstate"),
			Terraform:   &config.TerraformConfig{Source: "..."},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-b/module-b-child/"+config.DefaultConfigName)),
	}

	moduleC := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-c"),
		Dependencies: []*TerraformModule{moduleA},
		Config: config.TerragruntConfig{
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-a")}},
			Terraform:    &config.TerraformConfig{Source: "temp"},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-c/"+config.DefaultConfigName)),
	}

	moduleD := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-d"),
		Dependencies: []*TerraformModule{moduleA, moduleB, moduleC},
		Config: config.TerragruntConfig{
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-a"),
				Abs("../test/fixture-modules/module-b/module-b-child"), Abs("../test/fixture-modules/module-c")}},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-d/"+config.DefaultConfigName)),
	}

	configPaths := []string{"../test/fixture-modules/module-a/" + config.DefaultConfigName, "../test/fixture-modules/module-b/module-b-child/" + config.DefaultConfigName, "../test/fixture-modules/module-c/" + config.DefaultConfigName, "../test/fixture-modules/module-d/" + config.DefaultConfigName}
	expected := []*TerraformModule{moduleA, moduleB, moduleC, moduleD}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesMultipleModulesWithDependenciesWithIncludes(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              canonical(t, "../test/fixture-modules/module-a"),
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{Terraform: &config.TerraformConfig{Source: "test"}},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-a/"+config.DefaultConfigName)),
	}

	moduleB := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-b/module-b-child"),
		Dependencies: []*TerraformModule{},
		Config: config.TerragruntConfig{
			RemoteState: state(t, "bucket", "module-b-child/terraform.tfstate"),
			Terraform:   &config.TerraformConfig{Source: "..."},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-b/module-b-child/"+config.DefaultConfigName)),
	}

	moduleE := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-e/module-e-child"),
		Dependencies: []*TerraformModule{moduleA, moduleB},
		Config: config.TerragruntConfig{
			RemoteState:  state(t, "bucket", "module-e-child/terraform.tfstate"),
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-a"), Abs("../test/fixture-modules/module-b/module-b-child")}},
			Terraform:    &config.TerraformConfig{Source: "test"},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-e/module-e-child/"+config.DefaultConfigName)),
	}

	configPaths := []string{"../test/fixture-modules/module-a/" + config.DefaultConfigName, "../test/fixture-modules/module-b/module-b-child/" + config.DefaultConfigName, "../test/fixture-modules/module-e/module-e-child/" + config.DefaultConfigName}
	expected := []*TerraformModule{moduleA, moduleB, moduleE}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesMultipleModulesWithExternalDependencies(t *testing.T) {
	t.Parallel()

	moduleF := &TerraformModule{
		Path:                 canonical(t, "../test/fixture-modules/module-f"),
		Dependencies:         []*TerraformModule{},
		Config:               config.TerragruntConfig{},
		TerragruntOptions:    mockOptions.Clone(canonical(t, "../test/fixture-modules/module-f/"+config.DefaultConfigName)),
		AssumeAlreadyApplied: true,
	}

	moduleG := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-g"),
		Dependencies: []*TerraformModule{moduleF},
		Config: config.TerragruntConfig{
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-f")}},
			Terraform:    &config.TerraformConfig{Source: "test"},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-g/"+config.DefaultConfigName)),
	}

	configPaths := []string{"../test/fixture-modules/module-g/" + config.DefaultConfigName}
	expected := []*TerraformModule{moduleF, moduleG}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesMultipleModulesWithNestedExternalDependencies(t *testing.T) {
	t.Parallel()

	moduleH := &TerraformModule{
		Path:                 canonical(t, "../test/fixture-modules/module-h"),
		Dependencies:         []*TerraformModule{},
		Config:               config.TerragruntConfig{},
		TerragruntOptions:    mockOptions.Clone(canonical(t, "../test/fixture-modules/module-h/"+config.DefaultConfigName)),
		AssumeAlreadyApplied: true,
	}

	moduleI := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-i"),
		Dependencies: []*TerraformModule{moduleH},
		Config: config.TerragruntConfig{
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-h")}},
		},
		TerragruntOptions:    mockOptions.Clone(canonical(t, "../test/fixture-modules/module-i/"+config.DefaultConfigName)),
		AssumeAlreadyApplied: true,
	}

	moduleJ := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-j"),
		Dependencies: []*TerraformModule{moduleI},
		Config: config.TerragruntConfig{
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-i")}},
			Terraform:    &config.TerraformConfig{Source: "temp"},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-j/"+config.DefaultConfigName)),
	}

	moduleK := &TerraformModule{
		Path:         canonical(t, "../test/fixture-modules/module-k"),
		Dependencies: []*TerraformModule{moduleH},
		Config: config.TerragruntConfig{
			Dependencies: &config.ModuleDependencies{Paths: []string{Abs("../test/fixture-modules/module-h")}},
			Terraform:    &config.TerraformConfig{Source: "fire"},
		},
		TerragruntOptions: mockOptions.Clone(canonical(t, "../test/fixture-modules/module-k/"+config.DefaultConfigName)),
	}

	configPaths := []string{"../test/fixture-modules/module-j/" + config.DefaultConfigName, "../test/fixture-modules/module-k/" + config.DefaultConfigName}
	expected := []*TerraformModule{moduleH, moduleI, moduleJ, moduleK}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func TestResolveTerraformModulesInvalidPaths(t *testing.T) {
	t.Parallel()

	configPaths := []string{"../test/fixture-modules/module-missing-dependency/" + config.DefaultConfigName}

	_, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	if assert.NotNil(t, actualErr, "Unexpected error: %v", actualErr) {
		unwrapped := errors.Unwrap(actualErr)
		assert.True(t, os.IsNotExist(unwrapped), "Expected a file not exists error but got %v", unwrapped)
	}
}

func TestResolveTerraformModuleNoTerraformConfig(t *testing.T) {
	t.Parallel()

	configPaths := []string{"../test/fixture-modules/module-l/" + config.DefaultConfigName}
	expected := []*TerraformModule{}

	actualModules, actualErr := ResolveTerraformModules(configPaths, mockOptions)
	assert.Nil(t, actualErr, "Unexpected error: %v", actualErr)
	assertModuleListsEqual(t, expected, actualModules)
}

func Abs(path string) string {
	absPath, _ := filepath.Abs(path)
	return absPath
}
