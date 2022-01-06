package configstack

import (
	"fmt"
	"testing"

	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/stretchr/testify/assert"
)

var mockOptions = options.NewTerragruntOptionsForTest("running_module_test")

func TestToRunningModulesNoModules(t *testing.T) {
	t.Parallel()

	testToRunningModules(t, []*TerraformModule{}, NormalOrder, map[string]*runningModule{})
}

func TestToRunningModulesOneModuleNoDependencies(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleA := &runningModule{
		Module:         moduleA,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
	}

	modules := []*TerraformModule{moduleA}
	expected := map[string]*runningModule{"a": runningModuleA}

	testToRunningModules(t, modules, NormalOrder, expected)
}

func TestToRunningModulesTwoModulesNoDependencies(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleA := &runningModule{
		Module:         moduleA,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
	}

	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleB := &runningModule{
		Module:         moduleB,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
	}

	modules := []*TerraformModule{moduleA, moduleB}
	expected := map[string]*runningModule{"a": runningModuleA, "b": runningModuleB}

	testToRunningModules(t, modules, NormalOrder, expected)
}

func TestToRunningModulesTwoModulesWithDependencies(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleA := &runningModule{
		Module:         moduleA,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
	}

	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleB := &runningModule{
		Module:         moduleB,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{"a": runningModuleA},
		NotifyWhenDone: []*runningModule{},
	}

	runningModuleA.NotifyWhenDone = []*runningModule{runningModuleB}

	modules := []*TerraformModule{moduleA, moduleB}
	expected := map[string]*runningModule{"a": runningModuleA, "b": runningModuleB}

	testToRunningModules(t, modules, NormalOrder, expected)
}

func TestToRunningModulesTwoModulesWithDependenciesReverseOrder(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleA := &runningModule{
		Module:         moduleA,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
	}

	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleB := &runningModule{
		Module:         moduleB,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{runningModuleA},
	}

	runningModuleA.Dependencies = map[string]*runningModule{"b": runningModuleB}

	modules := []*TerraformModule{moduleA, moduleB}
	expected := map[string]*runningModule{"a": runningModuleA, "b": runningModuleB}

	testToRunningModules(t, modules, ReverseOrder, expected)
}

func TestToRunningModulesMultipleModulesWithAndWithoutDependencies(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleA := &runningModule{
		Module:         moduleA,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
	}

	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleB := &runningModule{
		Module:         moduleB,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{"a": runningModuleA},
		NotifyWhenDone: []*runningModule{},
	}

	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleC := &runningModule{
		Module:         moduleC,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{"a": runningModuleA},
		NotifyWhenDone: []*runningModule{},
	}

	moduleD := &TerraformModule{
		Path:              "d",
		Dependencies:      []*TerraformModule{moduleC},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleD := &runningModule{
		Module:         moduleD,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{"c": runningModuleC},
		NotifyWhenDone: []*runningModule{},
	}

	moduleE := &TerraformModule{
		Path:              "e",
		Dependencies:      []*TerraformModule{moduleA, moduleB, moduleC, moduleD},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleE := &runningModule{
		Module: moduleE,
		Status: waiting,
		Err:    nil,
		Dependencies: map[string]*runningModule{
			"a": runningModuleA,
			"b": runningModuleB,
			"c": runningModuleC,
			"d": runningModuleD,
		},
		NotifyWhenDone: []*runningModule{},
	}

	runningModuleA.NotifyWhenDone = []*runningModule{runningModuleB, runningModuleC, runningModuleE}
	runningModuleB.NotifyWhenDone = []*runningModule{runningModuleE}
	runningModuleC.NotifyWhenDone = []*runningModule{runningModuleD, runningModuleE}
	runningModuleD.NotifyWhenDone = []*runningModule{runningModuleE}

	modules := []*TerraformModule{moduleA, moduleB, moduleC, moduleD, moduleE}
	expected := map[string]*runningModule{
		"a": runningModuleA,
		"b": runningModuleB,
		"c": runningModuleC,
		"d": runningModuleD,
		"e": runningModuleE,
	}

	testToRunningModules(t, modules, NormalOrder, expected)
}

func TestToRunningModulesMultipleModulesWithAndWithoutDependenciesReverseOrder(t *testing.T) {
	t.Parallel()

	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleA := &runningModule{
		Module:         moduleA,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{},
	}

	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleB := &runningModule{
		Module:         moduleB,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{runningModuleA},
	}

	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleC := &runningModule{
		Module:         moduleC,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{runningModuleA},
	}

	moduleD := &TerraformModule{
		Path:              "d",
		Dependencies:      []*TerraformModule{moduleC},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleD := &runningModule{
		Module:         moduleD,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{runningModuleC},
	}

	moduleE := &TerraformModule{
		Path:              "e",
		Dependencies:      []*TerraformModule{moduleA, moduleB, moduleC, moduleD},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: mockOptions,
	}

	runningModuleE := &runningModule{
		Module:         moduleE,
		Status:         waiting,
		Err:            nil,
		Dependencies:   map[string]*runningModule{},
		NotifyWhenDone: []*runningModule{runningModuleA, runningModuleB, runningModuleC, runningModuleD},
	}

	runningModuleA.Dependencies = map[string]*runningModule{"b": runningModuleB, "c": runningModuleC, "e": runningModuleE}
	runningModuleB.Dependencies = map[string]*runningModule{"e": runningModuleE}
	runningModuleC.Dependencies = map[string]*runningModule{"d": runningModuleD, "e": runningModuleE}
	runningModuleD.Dependencies = map[string]*runningModule{"e": runningModuleE}

	modules := []*TerraformModule{moduleA, moduleB, moduleC, moduleD, moduleE}
	expected := map[string]*runningModule{
		"a": runningModuleA,
		"b": runningModuleB,
		"c": runningModuleC,
		"d": runningModuleD,
		"e": runningModuleE,
	}

	testToRunningModules(t, modules, ReverseOrder, expected)
}

func testToRunningModules(t *testing.T, modules []*TerraformModule, order dependencyOrder, expected map[string]*runningModule) {
	actual, err := toRunningModules(modules, order)
	if assert.Nil(t, err, "For modules %v and order %v", modules, order) {
		assertRunningModuleMapsEqual(t, expected, actual, true, "For modules %v and order %v", modules, order)
	}
}

func TestRunModulesNoModules(t *testing.T) {
	t.Parallel()

	err := runModules([]*TerraformModule{})
	assert.Nil(t, err, "Unexpected error: %v", err)
}

func TestRunModulesOneModuleSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	err := runModules([]*TerraformModule{moduleA})
	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.True(t, aRan)
}

func TestRunModulesOneModuleAssumeAlreadyRan(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:                 "a",
		Dependencies:         []*TerraformModule{},
		Config:               config.TerragruntConfig{},
		TerragruntOptions:    optionsWithMockTerragruntCommand("a", nil, &aRan),
		AssumeAlreadyApplied: true,
	}

	err := runModules([]*TerraformModule{moduleA})
	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.False(t, aRan)
}

func TestRunModulesReverseOrderOneModuleSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	err := runModulesReverseOrder([]*TerraformModule{moduleA})
	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.True(t, aRan)
}

func TestRunModulesOneModuleError(t *testing.T) {
	t.Parallel()

	aRan := false
	expectedErrA := fmt.Errorf("Expected error for module a")
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", expectedErrA, &aRan),
	}

	err := runModules([]*TerraformModule{moduleA})
	assertMultiErrorContains(t, err, expectedErrA)
	assert.True(t, aRan)
}

func TestRunModulesReverseOrderOneModuleError(t *testing.T) {
	t.Parallel()

	aRan := false
	expectedErrA := fmt.Errorf("Expected error for module a")
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", expectedErrA, &aRan),
	}

	err := runModulesReverseOrder([]*TerraformModule{moduleA})
	assertMultiErrorContains(t, err, expectedErrA)
	assert.True(t, aRan)
}

func TestRunModulesMultipleModulesNoDependenciesSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC})
	assert.Nil(t, err, "Unexpected error: %v", err)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesReverseOrderMultipleModulesNoDependenciesSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	err := runModulesReverseOrder([]*TerraformModule{moduleA, moduleB, moduleC})
	assert.Nil(t, err, "Unexpected error: %v", err)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesMultipleModulesNoDependenciesOneFailure(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	expectedErrB := fmt.Errorf("Expected error for module b")
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", expectedErrB, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC})
	assertMultiErrorContains(t, err, expectedErrB)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesMultipleModulesNoDependenciesMultipleFailures(t *testing.T) {
	t.Parallel()

	aRan := false
	expectedErrA := fmt.Errorf("Expected error for module a")
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", expectedErrA, &aRan),
	}

	bRan := false
	expectedErrB := fmt.Errorf("Expected error for module b")
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", expectedErrB, &bRan),
	}

	cRan := false
	expectedErrC := fmt.Errorf("Expected error for module c")
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", expectedErrC, &cRan),
	}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC})
	assertMultiErrorContains(t, err, expectedErrA, expectedErrB, expectedErrC)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesMultipleModulesWithDependenciesSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC})
	assert.Nil(t, err, "Unexpected error: %v", err)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesMultipleModulesWithDependenciesWithAssumeAlreadyRanSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:                 "c",
		Dependencies:         []*TerraformModule{moduleB},
		Config:               config.TerragruntConfig{},
		TerragruntOptions:    optionsWithMockTerragruntCommand("c", nil, &cRan),
		AssumeAlreadyApplied: true,
	}

	dRan := false
	moduleD := &TerraformModule{
		Path:              "d",
		Dependencies:      []*TerraformModule{moduleC},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("d", nil, &dRan),
	}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC, moduleD})
	assert.Nil(t, err, "Unexpected error: %v", err)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.False(t, cRan)
	assert.True(t, dRan)
}

func TestRunModulesReverseOrderMultipleModulesWithDependenciesSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	err := runModulesReverseOrder([]*TerraformModule{moduleA, moduleB, moduleC})
	assert.Nil(t, err, "Unexpected error: %v", err)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesMultipleModulesWithDependenciesOneFailure(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	expectedErrB := fmt.Errorf("Expected error for module b")
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", expectedErrB, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	expectedErrC := dependencyFinishedWithError{moduleC, moduleB, expectedErrB}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC})
	assertMultiErrorContains(t, err, expectedErrB, expectedErrC)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.False(t, cRan)
}

func TestRunModulesMultipleModulesWithDependenciesOneFailureIgnoreDependencyErrors(t *testing.T) {
	t.Parallel()

	aRan := false
	terragruntOptionsA := optionsWithMockTerragruntCommand("a", nil, &aRan)
	terragruntOptionsA.IgnoreDependencyErrors = true
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: terragruntOptionsA,
	}

	bRan := false
	expectedErrB := fmt.Errorf("Expected error for module b")
	terragruntOptionsB := optionsWithMockTerragruntCommand("b", expectedErrB, &bRan)
	terragruntOptionsB.IgnoreDependencyErrors = true
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: terragruntOptionsB,
	}

	cRan := false
	terragruntOptionsC := optionsWithMockTerragruntCommand("c", nil, &cRan)
	terragruntOptionsC.IgnoreDependencyErrors = true
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: terragruntOptionsC,
	}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC})
	assertMultiErrorContains(t, err, expectedErrB)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesReverseOrderMultipleModulesWithDependenciesOneFailure(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	expectedErrB := fmt.Errorf("Expected error for module b")
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", expectedErrB, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	expectedErrA := dependencyFinishedWithError{moduleA, moduleB, expectedErrB}

	err := runModulesReverseOrder([]*TerraformModule{moduleA, moduleB, moduleC})
	assertMultiErrorContains(t, err, expectedErrB, expectedErrA)

	assert.False(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
}

func TestRunModulesMultipleModulesWithDependenciesMultipleFailures(t *testing.T) {
	t.Parallel()

	aRan := false
	expectedErrA := fmt.Errorf("Expected error for module a")
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", expectedErrA, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	expectedErrB := dependencyFinishedWithError{moduleB, moduleA, expectedErrA}
	expectedErrC := dependencyFinishedWithError{moduleC, moduleB, expectedErrB}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC})
	assertMultiErrorContains(t, err, expectedErrA, expectedErrB, expectedErrC)

	assert.True(t, aRan)
	assert.False(t, bRan)
	assert.False(t, cRan)
}

func TestRunModulesMultipleModulesWithDependenciesLargeGraphAllSuccess(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", nil, &cRan),
	}

	dRan := false
	moduleD := &TerraformModule{
		Path:              "d",
		Dependencies:      []*TerraformModule{moduleA, moduleB, moduleC},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("d", nil, &dRan),
	}

	eRan := false
	moduleE := &TerraformModule{
		Path:              "e",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("e", nil, &eRan),
	}

	fRan := false
	moduleF := &TerraformModule{
		Path:              "f",
		Dependencies:      []*TerraformModule{moduleE, moduleD},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("f", nil, &fRan),
	}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC, moduleD, moduleE, moduleF})
	assert.Nil(t, err)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
	assert.True(t, dRan)
	assert.True(t, eRan)
	assert.True(t, fRan)
}

func TestRunModulesMultipleModulesWithDependenciesLargeGraphPartialFailure(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "large-graph-a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("large-graph-a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "large-graph-b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("large-graph-b", nil, &bRan),
	}

	cRan := false
	expectedErrC := fmt.Errorf("Expected error for module large-graph-c")
	moduleC := &TerraformModule{
		Path:              "large-graph-c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("large-graph-c", expectedErrC, &cRan),
	}

	dRan := false
	moduleD := &TerraformModule{
		Path:              "large-graph-d",
		Dependencies:      []*TerraformModule{moduleA, moduleB, moduleC},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("large-graph-d", nil, &dRan),
	}

	eRan := false
	moduleE := &TerraformModule{
		Path:                 "large-graph-e",
		Dependencies:         []*TerraformModule{},
		Config:               config.TerragruntConfig{},
		TerragruntOptions:    optionsWithMockTerragruntCommand("large-graph-e", nil, &eRan),
		AssumeAlreadyApplied: true,
	}

	fRan := false
	moduleF := &TerraformModule{
		Path:              "large-graph-f",
		Dependencies:      []*TerraformModule{moduleE, moduleD},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("large-graph-f", nil, &fRan),
	}

	gRan := false
	moduleG := &TerraformModule{
		Path:              "large-graph-g",
		Dependencies:      []*TerraformModule{moduleE},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("large-graph-g", nil, &gRan),
	}

	expectedErrD := dependencyFinishedWithError{moduleD, moduleC, expectedErrC}
	expectedErrF := dependencyFinishedWithError{moduleF, moduleD, expectedErrD}

	err := runModules([]*TerraformModule{moduleA, moduleB, moduleC, moduleD, moduleE, moduleF, moduleG})
	assertMultiErrorContains(t, err, expectedErrC, expectedErrD, expectedErrF)

	assert.True(t, aRan)
	assert.True(t, bRan)
	assert.True(t, cRan)
	assert.False(t, dRan)
	assert.False(t, eRan)
	assert.False(t, fRan)
	assert.True(t, gRan)
}

func TestRunModulesReverseOrderMultipleModulesWithDependenciesLargeGraphPartialFailure(t *testing.T) {
	t.Parallel()

	aRan := false
	moduleA := &TerraformModule{
		Path:              "a",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("a", nil, &aRan),
	}

	bRan := false
	moduleB := &TerraformModule{
		Path:              "b",
		Dependencies:      []*TerraformModule{moduleA},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("b", nil, &bRan),
	}

	cRan := false
	expectedErrC := fmt.Errorf("Expected error for module c")
	moduleC := &TerraformModule{
		Path:              "c",
		Dependencies:      []*TerraformModule{moduleB},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("c", expectedErrC, &cRan),
	}

	dRan := false
	moduleD := &TerraformModule{
		Path:              "d",
		Dependencies:      []*TerraformModule{moduleA, moduleB, moduleC},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("d", nil, &dRan),
	}

	eRan := false
	moduleE := &TerraformModule{
		Path:              "e",
		Dependencies:      []*TerraformModule{},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("e", nil, &eRan),
	}

	fRan := false
	moduleF := &TerraformModule{
		Path:              "f",
		Dependencies:      []*TerraformModule{moduleE, moduleD},
		Config:            config.TerragruntConfig{},
		TerragruntOptions: optionsWithMockTerragruntCommand("f", nil, &fRan),
	}

	expectedErrB := dependencyFinishedWithError{moduleB, moduleC, expectedErrC}
	expectedErrA := dependencyFinishedWithError{moduleA, moduleB, expectedErrB}

	err := runModulesReverseOrder([]*TerraformModule{moduleA, moduleB, moduleC, moduleD, moduleE, moduleF})
	assertMultiErrorContains(t, err, expectedErrC, expectedErrB, expectedErrA)

	assert.False(t, aRan)
	assert.False(t, bRan)
	assert.True(t, cRan)
	assert.True(t, dRan)
	assert.True(t, eRan)
	assert.True(t, fRan)
}
