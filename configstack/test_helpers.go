package configstack

import (
	"sort"
	"testing"

	"github.com/coveooss/terragrunt/v2/config"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/remote"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

type terraformModuleByPath []*TerraformModule

func (byPath terraformModuleByPath) Len() int           { return len(byPath) }
func (byPath terraformModuleByPath) Swap(i, j int)      { byPath[i], byPath[j] = byPath[j], byPath[i] }
func (byPath terraformModuleByPath) Less(i, j int) bool { return byPath[i].Path < byPath[j].Path }

type runningModuleByPath []*runningModule

func (byPath runningModuleByPath) Len() int      { return len(byPath) }
func (byPath runningModuleByPath) Swap(i, j int) { byPath[i], byPath[j] = byPath[j], byPath[i] }
func (byPath runningModuleByPath) Less(i, j int) bool {
	return byPath[i].Module.Path < byPath[j].Module.Path
}

// We can't use assert.Equals on TerraformModule or any data structure that contains it because it contains some
// fields (e.g. TerragruntOptions) that cannot be compared directly
func assertModuleListsEqual(t *testing.T, expectedModules []*TerraformModule, actualModules []*TerraformModule, messageAndArgs ...interface{}) {
	if !assert.Equal(t, len(expectedModules), len(actualModules), messageAndArgs...) {
		t.Logf("%s != %s", expectedModules, actualModules)
		return
	}

	sort.Sort(terraformModuleByPath(expectedModules))
	sort.Sort(terraformModuleByPath(actualModules))

	for i := 0; i < len(expectedModules); i++ {
		expected := expectedModules[i]
		actual := actualModules[i]
		assertModulesEqual(t, expected, actual, messageAndArgs...)
	}
}

// We can't use assert.Equals on TerraformModule because it contains some fields (e.g. TerragruntOptions) that cannot
// be compared directly
func assertModulesEqual(t *testing.T, expected *TerraformModule, actual *TerraformModule, messageAndArgs ...interface{}) {
	if assert.NotNil(t, actual, messageAndArgs...) {
		assert.Equal(t, expected.Path, actual.Path, messageAndArgs...)
		assert.Equal(t, expected.AssumeAlreadyApplied, actual.AssumeAlreadyApplied, messageAndArgs...)

		assertConfigsEqual(t, expected.Config, actual.Config, messageAndArgs...)
		assertOptionsEqual(t, *expected.TerragruntOptions, *actual.TerragruntOptions, messageAndArgs...)
		assertModuleListsEqual(t, expected.Dependencies, actual.Dependencies, messageAndArgs...)
	}
}

// We can't use assert.Equals on TerraformModule or any data structure that contains it (e.g. runningModule) because it
// contains some fields (e.g. TerragruntOptions) that cannot be compared directly
func assertRunningModuleMapsEqual(t *testing.T, expectedModules map[string]*runningModule, actualModules map[string]*runningModule, doDeepCheck bool, messageAndArgs ...interface{}) {
	if !assert.Equal(t, len(expectedModules), len(actualModules), messageAndArgs...) {
		t.Logf("%v != %v", expectedModules, actualModules)
		return
	}

	for expectedPath, expectedModule := range expectedModules {
		actualModule, containsModule := actualModules[expectedPath]
		if assert.True(t, containsModule, messageAndArgs...) {
			assertRunningModulesEqual(t, expectedModule, actualModule, doDeepCheck, messageAndArgs...)
		}
	}
}

// We can't use assert.Equals on TerraformModule or any data structure that contains it (e.g. runningModule) because it
// contains some fields (e.g. TerragruntOptions) that cannot be compared directly
func assertRunningModuleListsEqual(t *testing.T, expectedModules []*runningModule, actualModules []*runningModule, doDeepCheck bool, messageAndArgs ...interface{}) {
	if !assert.Equal(t, len(expectedModules), len(actualModules), messageAndArgs...) {
		t.Logf("%v != %v", expectedModules, actualModules)
		return
	}

	sort.Sort(runningModuleByPath(expectedModules))
	sort.Sort(runningModuleByPath(actualModules))

	for i := 0; i < len(expectedModules); i++ {
		expected := expectedModules[i]
		actual := actualModules[i]
		assertRunningModulesEqual(t, expected, actual, doDeepCheck, messageAndArgs...)
	}
}

// We can't use assert.Equals on TerraformModule or any data structure that contains it (e.g. runningModule) because it
// contains some fields (e.g. TerragruntOptions) that cannot be compared directly
func assertRunningModulesEqual(t *testing.T, expected *runningModule, actual *runningModule, doDeepCheck bool, messageAndArgs ...interface{}) {
	if assert.NotNil(t, actual, messageAndArgs...) {
		assert.Equal(t, expected.Status, actual.Status, messageAndArgs...)

		assertModulesEqual(t, expected.Module, actual.Module, messageAndArgs...)
		assertErrorsEqual(t, expected.Err, actual.Err, messageAndArgs...)

		// This ensures we don't end up in a circular loop, since there is a (intentional) circular dependency
		// between NotifyWhenDone and Dependencies
		if doDeepCheck {
			assertRunningModuleMapsEqual(t, expected.Dependencies, actual.Dependencies, false, messageAndArgs...)
			assertRunningModuleListsEqual(t, expected.NotifyWhenDone, actual.NotifyWhenDone, false, messageAndArgs...)
		}
	}
}

// We can't do a simple IsError comparison for UnrecognizedDependency because that error is a struct that
// contains an array, and in Go, trying to compare arrays gives a "comparing non comparable type
// configstack.UnrecognizedDependency" panic. Therefore, we have to compare that error more manually.
func assertErrorsEqual(t *testing.T, expected error, actual error, messageAndArgs ...interface{}) {
	actual = tgerrors.Unwrap(actual)
	if expectedUnrecognized, isUnrecognizedDependencyError := expected.(UnrecognizedDependency); isUnrecognizedDependencyError {
		actualUnrecognized, isUnrecognizedDependencyError := actual.(UnrecognizedDependency)
		if assert.True(t, isUnrecognizedDependencyError, messageAndArgs...) {
			assert.Equal(t, expectedUnrecognized, actualUnrecognized, messageAndArgs...)
		}
	} else {
		assert.True(t, tgerrors.IsError(actual, expected), messageAndArgs...)
	}
}

// We can't do a direct comparison between TerragruntOptions objects because we can't compare Logger or RunTerragrunt
// instances. Therefore, we have to manually check everything else.
func assertOptionsEqual(t *testing.T, expected options.TerragruntOptions, actual options.TerragruntOptions, messageAndArgs ...interface{}) {
	assert.NotNil(t, expected.Logger, messageAndArgs...)
	assert.NotNil(t, actual.Logger, messageAndArgs...)

	assert.Equal(t, expected.TerragruntConfigPath, actual.TerragruntConfigPath, messageAndArgs...)
	assert.Equal(t, expected.NonInteractive, actual.NonInteractive, messageAndArgs...)
	assert.Equal(t, expected.TerraformCliArgs, actual.TerraformCliArgs, messageAndArgs...)
	assert.Equal(t, expected.WorkingDir, actual.WorkingDir, messageAndArgs...)
}

// We can't do a direct comparison between TerragruntConfig objects because they contain option objects.
func assertConfigsEqual(t *testing.T, expected config.TerragruntConfig, actual config.TerragruntConfig, messageAndArgs ...interface{}) {
	if actual.RemoteState != nil {
		actual.RemoteState.ConfigHclDefinition = cty.NilVal
	}
	assert.Equal(t, expected.ApprovalConfig, actual.ApprovalConfig, messageAndArgs...)
	assert.Equal(t, expected.AssumeRole, actual.AssumeRole, messageAndArgs...)
	assert.Equal(t, expected.Dependencies, actual.Dependencies, messageAndArgs...)
	assert.Equal(t, expected.Description, actual.Description, messageAndArgs...)
	assert.Equal(t, expected.ExtraCommands, actual.ExtraCommands, messageAndArgs...)
	assert.Equal(t, expected.ImportFiles, actual.ImportFiles, messageAndArgs...)
	assert.Equal(t, expected.PostHooks, actual.PostHooks, messageAndArgs...)
	assert.Equal(t, expected.PreHooks, actual.PreHooks, messageAndArgs...)
	assert.Equal(t, expected.RemoteState, actual.RemoteState, messageAndArgs...)
	assert.Equal(t, expected.Terraform, actual.Terraform, messageAndArgs...)
	assert.Equal(t, expected.UniquenessCriteria, actual.UniquenessCriteria, messageAndArgs...)
}

// Return the absolute path for the given path
func canonical(t *testing.T, path string) string {
	out, err := util.CanonicalPath(path, ".")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Create a State struct
func state(bucket string, key string) *remote.State {
	return &remote.State{
		Backend: "s3",
		Config: map[string]interface{}{
			"bucket": bucket,
			"key":    key,
		},
	}
}

// Create a mock TerragruntOptions object and configure its RunTerragrunt command to return the given error object. If
// the RunTerragrunt command is called, this method will also set the executed boolean to true.
func optionsWithMockTerragruntCommand(terragruntConfigPath string, toReturnFromTerragruntCommand error, executed *bool) *options.TerragruntOptions {
	opts := options.NewTerragruntOptionsForTest(terragruntConfigPath)
	opts.RunTerragrunt = func(_ *options.TerragruntOptions) error {
		*executed = true
		return toReturnFromTerragruntCommand
	}
	return opts
}

func assertMultiErrorContains(t *testing.T, actualError error, expectedErrors ...error) {
	actualError = tgerrors.Unwrap(actualError)
	errMulti, isMultiError := actualError.(errMulti)
	if assert.True(t, isMultiError, "Expected a MultiError, but got: %v", actualError) {
		assert.Equal(t, len(expectedErrors), len(errMulti.Errors))
		for _, expectedErr := range expectedErrors {
			found := false
			for _, actualErr := range errMulti.Errors {
				if expectedErr == actualErr {
					found = true
					break
				}
			}
			assert.True(t, found, "Couldn't find expected error %v", expectedErr)
		}
	}
}
