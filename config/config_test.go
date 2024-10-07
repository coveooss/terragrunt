package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/remote"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/stretchr/testify/assert"
)

var mockOptions = options.NewTerragruntOptionsForTest("test-time-mock")

func TestParseTerragruntConfigRemoteStateMinimalConfig(t *testing.T) {
	t.Parallel()

	config := `
		remote_state {
			backend = "s3"
		}
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.Terraform)

	if assert.NotNil(t, terragruntConfig.RemoteState) {
		assert.Equal(t, "s3", terragruntConfig.RemoteState.Backend)
		assert.Empty(t, terragruntConfig.RemoteState.Config)
	}
}

func TestParseTerragruntConfigHooks(t *testing.T) {
	config := `
		pre_hook "pre_hook_1" {
			command = "echo hello from pre_hook 1"
		}
		pre_hook "pre_hook_2" {
			command = "echo hello from pre_hook 2"
		}
		post_hook "post_hook_1" {
			command = "echo hello from post_hook 1"
		}
		post_hook "post_hook_2" {
			command = "echo hello from post_hook 2"
		}
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	// During these tests, we don't care about the config references that are sprinkled everywhere
	for idx := range terragruntConfig.PreHooks {
		terragruntConfig.PreHooks[idx]._config = nil
	}
	for idx := range terragruntConfig.PostHooks {
		terragruntConfig.PostHooks[idx]._config = nil
	}

	assert.Equal(t,
		HookList{
			Hook{
				TerragruntExtensionBase: TerragruntExtensionBase{
					Name: "pre_hook_1",
				},
				Type:    PreHookType,
				Command: "echo hello from pre_hook 1",
			},
			Hook{
				TerragruntExtensionBase: TerragruntExtensionBase{
					Name: "pre_hook_2",
				},
				Type:    PreHookType,
				Command: "echo hello from pre_hook 2",
			},
		},
		terragruntConfig.PreHooks,
	)
	assert.Equal(t,
		HookList{
			Hook{
				TerragruntExtensionBase: TerragruntExtensionBase{
					Name: "post_hook_1",
				},
				Type:    PostHookType,
				Command: "echo hello from post_hook 1",
			},
			Hook{
				TerragruntExtensionBase: TerragruntExtensionBase{
					Name: "post_hook_2",
				},
				Type:    PostHookType,
				Command: "echo hello from post_hook 2",
			},
		},
		terragruntConfig.PostHooks,
	)
}

func TestParseTerragruntConfigRemoteStateMissingBackend(t *testing.T) {
	t.Parallel()

	config := `
		remote_state {
		}
	`

	_, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	assert.EqualError(t, err, "caught error while initializing the Terragrunt config: the remote_state.backend field cannot be empty")
}

func TestParseTerragruntConfigRemoteStateFullConfig(t *testing.T) {
	t.Parallel()

	config := `
		remote_state {
			backend = "s3"
			config = {
				encrypt = true
				bucket = "my-bucket"
				key = "terraform.tfstate"
				region = "us-east-1"
			}
		}
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.Terraform)

	if assert.NotNil(t, terragruntConfig.RemoteState) {
		assert.Equal(t, "s3", terragruntConfig.RemoteState.Backend)
		assert.NotEmpty(t, terragruntConfig.RemoteState.Config)
		assert.Equal(t, true, terragruntConfig.RemoteState.Config["encrypt"])
		assert.Equal(t, "my-bucket", terragruntConfig.RemoteState.Config["bucket"])
		assert.Equal(t, "terraform.tfstate", terragruntConfig.RemoteState.Config["key"])
		assert.Equal(t, "us-east-1", terragruntConfig.RemoteState.Config["region"])
	}
}

func TestParseTerragruntConfigDependenciesOnePath(t *testing.T) {
	t.Parallel()

	config := `
		dependencies {
			paths = ["../vpc"]
		}
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.RemoteState)
	assert.Nil(t, terragruntConfig.Terraform)

	if assert.NotNil(t, terragruntConfig.Dependencies) {
		assert.Equal(t, []string{"../vpc"}, terragruntConfig.Dependencies.Paths)
	}
}

func TestParseTerragruntConfigDependenciesMultiplePaths(t *testing.T) {
	t.Parallel()

	config := `
		dependencies {
			paths = ["../vpc", "../mysql", "../backend-app"]
		}
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.RemoteState)
	assert.Nil(t, terragruntConfig.Terraform)

	if assert.NotNil(t, terragruntConfig.Dependencies) {
		assert.Equal(t, []string{"../vpc", "../mysql", "../backend-app"}, terragruntConfig.Dependencies.Paths)
	}
}

func TestParseTerragruntConfigRemoteStateDynamoDbTerraformConfigAndDependenciesFullConfig(t *testing.T) {
	t.Parallel()

	config := `
		terraform {
			source = "foo"
		}

		remote_state {
			backend = "s3"
			config = {
				encrypt = true
				bucket = "my-bucket"
				key = "terraform.tfstate"
				region = "us-east-1"
			}
		}

		dependencies {
			paths = ["../vpc", "../mysql", "../backend-app"]
		}
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	if assert.NotNil(t, terragruntConfig.Terraform) {
		assert.Equal(t, "foo", terragruntConfig.Terraform.Source)
	}

	if assert.NotNil(t, terragruntConfig.RemoteState) {
		assert.Equal(t, "s3", terragruntConfig.RemoteState.Backend)
		assert.NotEmpty(t, terragruntConfig.RemoteState.Config)
		assert.Equal(t, true, terragruntConfig.RemoteState.Config["encrypt"])
		assert.Equal(t, "my-bucket", terragruntConfig.RemoteState.Config["bucket"])
		assert.Equal(t, "terraform.tfstate", terragruntConfig.RemoteState.Config["key"])
		assert.Equal(t, "us-east-1", terragruntConfig.RemoteState.Config["region"])
	}

	if assert.NotNil(t, terragruntConfig.Dependencies) {
		assert.Equal(t, []string{"../vpc", "../mysql", "../backend-app"}, terragruntConfig.Dependencies.Paths)
	}
}

func TestParseTerragruntConfigInclude(t *testing.T) {
	t.Parallel()

	config := fmt.Sprintf(`
		include {
			path = "../../../%s"
		}
	`, DefaultConfigName)

	opts := options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/sub-sub-child/" + DefaultConfigName)
	terragruntConfig, err := parseConfigString(config, opts, IncludeConfig{Path: opts.TerragruntConfigPath})
	if assert.Nil(t, err, "Unexpected error: %v", tgerrors.PrintErrorWithStackTrace(err)) {
		assert.Nil(t, terragruntConfig.Terraform)

		if assert.NotNil(t, terragruntConfig.RemoteState) {
			assert.Equal(t, "s3", terragruntConfig.RemoteState.Backend)
			assert.NotEmpty(t, terragruntConfig.RemoteState.Config)
			assert.Equal(t, true, terragruntConfig.RemoteState.Config["encrypt"])
			assert.Equal(t, "my-bucket", terragruntConfig.RemoteState.Config["bucket"])
			assert.Equal(t, "child/sub-child/sub-sub-child/terraform.tfstate", terragruntConfig.RemoteState.Config["key"])
			assert.Equal(t, "us-east-1", terragruntConfig.RemoteState.Config["region"])
		}
	}
}

func TestParseTerragruntConfigIncludeWithFindInParentFolders(t *testing.T) {
	t.Parallel()

	config := `
		include {
			path = find_in_parent_folders()
		}
	`

	opts := options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/sub-sub-child/" + DefaultConfigName)
	terragruntConfig, err := parseConfigString(config, opts, IncludeConfig{Path: opts.TerragruntConfigPath})
	if assert.Nil(t, err, "Unexpected error: %v", tgerrors.PrintErrorWithStackTrace(err)) {
		assert.Nil(t, terragruntConfig.Terraform)

		if assert.NotNil(t, terragruntConfig.RemoteState) {
			assert.Equal(t, "s3", terragruntConfig.RemoteState.Backend)
			assert.NotEmpty(t, terragruntConfig.RemoteState.Config)
			assert.Equal(t, true, terragruntConfig.RemoteState.Config["encrypt"])
			assert.Equal(t, "my-bucket", terragruntConfig.RemoteState.Config["bucket"])
			assert.Equal(t, "child/sub-child/sub-sub-child/terraform.tfstate", terragruntConfig.RemoteState.Config["key"])
			assert.Equal(t, "us-east-1", terragruntConfig.RemoteState.Config["region"])
		}
	}
}

func TestParseTerragruntConfigIncludeOverrideRemote(t *testing.T) {
	t.Parallel()

	config := fmt.Sprintf(`
		  include {
		    path = "../../../%s"
		  }

		  # Configure Terragrunt to automatically store tfstate files in an S3 bucket
		  remote_state {
		    backend = "s3"
		    config = {
		      encrypt = false
		      bucket = "override"
		      key = "override"
		      region = "override"
		    }
		  }
	`, DefaultConfigName)

	opts := options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/sub-sub-child/" + DefaultConfigName)
	terragruntConfig, err := parseConfigString(config, opts, IncludeConfig{Path: opts.TerragruntConfigPath})
	if assert.Nil(t, err, "Unexpected error: %v", tgerrors.PrintErrorWithStackTrace(err)) {
		assert.Nil(t, terragruntConfig.Terraform)

		if assert.NotNil(t, terragruntConfig.RemoteState) {
			assert.Equal(t, "s3", terragruntConfig.RemoteState.Backend)
			assert.NotEmpty(t, terragruntConfig.RemoteState.Config)
			assert.Equal(t, false, terragruntConfig.RemoteState.Config["encrypt"])
			assert.Equal(t, "override", terragruntConfig.RemoteState.Config["bucket"])
			assert.Equal(t, "override", terragruntConfig.RemoteState.Config["key"])
			assert.Equal(t, "override", terragruntConfig.RemoteState.Config["region"])
		}
	}
}

func TestParseTerragruntConfigIncludeOverrideAll(t *testing.T) {
	t.Parallel()

	config := fmt.Sprintf(`
		  include {
		    path = "../../../%s"
		  }

		  terraform {
		    source = "foo"
		  }

		  # Configure Terragrunt to automatically store tfstate files in an S3 bucket
		  remote_state {
		    backend = "s3"
		    config = {
		      encrypt = false
		      bucket = "override"
		      key = "override"
		      region = "override"
		    }
		  }

		  dependencies {
		    paths = ["override"]
		  }
	`, DefaultConfigName)

	opts := options.NewTerragruntOptionsForTest("../test/fixture-parent-folders/terragrunt-in-root/child/sub-child/sub-sub-child/" + DefaultConfigName)
	terragruntConfig, err := parseConfigString(config, opts, IncludeConfig{Path: opts.TerragruntConfigPath})
	if assert.Nil(t, err, "Unexpected error: %v", tgerrors.PrintErrorWithStackTrace(err)) {
		if assert.NotNil(t, terragruntConfig.Terraform) {
			assert.Equal(t, "foo", terragruntConfig.Terraform.Source)
		}

		if assert.NotNil(t, terragruntConfig.RemoteState) {
			assert.Equal(t, "s3", terragruntConfig.RemoteState.Backend)
			assert.NotEmpty(t, terragruntConfig.RemoteState.Config)
			assert.Equal(t, false, terragruntConfig.RemoteState.Config["encrypt"])
			assert.Equal(t, "override", terragruntConfig.RemoteState.Config["bucket"])
			assert.Equal(t, "override", terragruntConfig.RemoteState.Config["key"])
			assert.Equal(t, "override", terragruntConfig.RemoteState.Config["region"])
		}

		assert.Equal(t, []string{"override"}, terragruntConfig.Dependencies.Paths)
	}
}

func TestParseTerragruntConfigTwoLevels(t *testing.T) {
	t.Parallel()

	configPath := "../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/sub-child/" + DefaultConfigName

	config, err := util.ReadFileAsString(configPath)
	if err != nil {
		t.Fatal(err)
	}

	terragruntConfig, err := parseConfigString(config, options.NewTerragruntOptionsForTest(configPath), IncludeConfig{Path: configPath})
	assert.Nil(t, err)
	assert.NotNil(t, terragruntConfig)
}

func TestParseTerragruntConfigThreeLevels(t *testing.T) {
	t.Parallel()

	configPath := "../test/fixture-parent-folders/multiple-terragrunt-in-parents/child/sub-child/sub-sub-child/" + DefaultConfigName

	config, err := util.ReadFileAsString(configPath)
	if err != nil {
		t.Fatal(err)
	}

	terragruntConfig, err := parseConfigString(config, options.NewTerragruntOptionsForTest(configPath), IncludeConfig{Path: configPath})
	assert.Nil(t, err)
	assert.NotNil(t, terragruntConfig)
}

func TestParseWithBootStrapFile(t *testing.T) {
	t.Parallel()

	fixture := "../test/fixture-bootstrap/simple/"
	configPath := fixture + DefaultConfigName
	config, err := util.ReadFileAsString(configPath)
	if err != nil {
		t.Fatal(err)
	}

	terragruntOptions := options.NewTerragruntOptionsForTest(configPath)
	absolute, _ := filepath.Abs(fixture)
	terragruntOptions.BootConfigurationPaths = []string{absolute + "/a.hcl", absolute + "/b.hcl"}
	terragruntConfig, err := parseConfigString(config, terragruntOptions, IncludeConfig{Path: configPath})
	assert.Nil(t, err)
	assert.NotNil(t, terragruntConfig)
	assert.Equal(t, len(terragruntConfig.PreHooks), 2, "Should have 2 pre_hook(s)")
}

func TestParseWithNoFile(t *testing.T) {
	t.Parallel()
	config, err := ReadTerragruntConfig(options.NewTerragruntOptionsForTest("../test/fixture-noconfig/no-file/" + DefaultConfigName))
	assert.Nil(t, err)
	assert.NotNil(t, config)
}

func TestParseWithNoConfig(t *testing.T) {
	t.Parallel()
	config, err := ReadTerragruntConfig(options.NewTerragruntOptionsForTest("../test/fixture-noconfig/no-terragrunt/" + DefaultConfigName))
	assert.Nil(t, err)
	assert.NotNil(t, config)
}

func TestParseWithBadPath(t *testing.T) {
	t.Parallel()
	config, err := ReadTerragruntConfig(options.NewTerragruntOptionsForTest("../test/fixture-noconfig/bad-path/" + DefaultConfigName))
	assert.NotNil(t, err)
	assert.Nil(t, config)
}

func TestParseValid(t *testing.T) {
	t.Parallel()
	config, err := ReadTerragruntConfig(options.NewTerragruntOptionsForTest("../test/fixture-noconfig/valid/" + DefaultConfigName))
	assert.Nil(t, err)
	assert.NotNil(t, config)
}

func TestParseInvalid(t *testing.T) {
	t.Parallel()
	config, err := ReadTerragruntConfig(options.NewTerragruntOptionsForTest("../test/fixture-noconfig/invalid/" + DefaultConfigName + ".invalid"))
	assert.NotNil(t, err)
	assert.Nil(t, config)
}

func TestReadTerragruntConfigHooksAreInitialized(t *testing.T) {
	t.Parallel()

	fixturesDir := "../test/fixture-hooks"

	files, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			fixturePath := path.Join(fixturesDir, file.Name())
			t.Run(fixturePath, func(t *testing.T) {
				config, err := ReadTerragruntConfig(options.NewTerragruntOptionsForTest(path.Join(fixturePath, DefaultConfigName)))
				assert.Nil(t, err)

				for _, hook := range config.PreHooks {
					assert.Equal(t, PreHookType, hook.Type)
				}
				for _, hook := range config.PostHooks {
					assert.Equal(t, PostHookType, hook.Type)
				}
			})
		}
	}
}

type argConfig struct {
	name      string
	extraArgs []string
}

func getExtraArgsConfig(options *options.TerragruntOptions, argConfigs ...argConfig) TerragruntConfig {
	args := []TerraformExtraArguments{}
	for _, argConfig := range argConfigs {
		base := TerragruntExtensionBase{Name: argConfig.name}
		base.init(&TerragruntConfigFile{})
		base.config().options = options
		args = append(args, TerraformExtraArguments{TerragruntExtensionBase: base, Arguments: argConfig.extraArgs})
	}
	return TerragruntConfig{ExtraArgs: args}
}

func TestMergeConfigIntoIncludedConfig(t *testing.T) {
	t.Parallel()

	options := options.NewTerragruntOptionsForTest("TestMergeConfigIntoIncludedConfig")

	testCases := []struct {
		config         TerragruntConfig
		includedConfig TerragruntConfig
		expected       TerragruntConfig
	}{
		{
			TerragruntConfig{},
			TerragruntConfig{},
			TerragruntConfig{},
		},
		{
			TerragruntConfig{},
			TerragruntConfig{Terraform: &TerraformConfig{Source: "foo"}},
			TerragruntConfig{Terraform: &TerraformConfig{Source: "foo"}},
		},
		{
			TerragruntConfig{},
			TerragruntConfig{RemoteState: &remote.State{Backend: "bar"}, Terraform: &TerraformConfig{Source: "foo"}},
			TerragruntConfig{RemoteState: &remote.State{Backend: "bar"}, Terraform: &TerraformConfig{Source: "foo"}},
		},
		{
			TerragruntConfig{RemoteState: &remote.State{Backend: "foo"}, Terraform: &TerraformConfig{Source: "foo"}},
			TerragruntConfig{RemoteState: &remote.State{Backend: "bar"}, Terraform: &TerraformConfig{Source: "bar"}},
			TerragruntConfig{RemoteState: &remote.State{Backend: "foo"}, Terraform: &TerraformConfig{Source: "foo"}},
		},
		{
			TerragruntConfig{Terraform: &TerraformConfig{Source: "foo"}},
			TerragruntConfig{RemoteState: &remote.State{Backend: "bar"}, Terraform: &TerraformConfig{Source: "bar"}},
			TerragruntConfig{RemoteState: &remote.State{Backend: "bar"}, Terraform: &TerraformConfig{Source: "foo"}},
		},
		{
			getExtraArgsConfig(options, argConfig{name: "childArgs"}),
			getExtraArgsConfig(options),
			getExtraArgsConfig(options, argConfig{name: "childArgs"}),
		},
		{
			getExtraArgsConfig(options, argConfig{name: "childArgs"}),
			getExtraArgsConfig(options, argConfig{name: "parentArgs"}),
			getExtraArgsConfig(options, argConfig{name: "parentArgs"}, argConfig{name: "childArgs"}),
		},
		{
			getExtraArgsConfig(options, argConfig{name: "overrideArgs", extraArgs: []string{"-child"}}),
			getExtraArgsConfig(options, argConfig{name: "overrideArgs", extraArgs: []string{"-parent"}}),
			getExtraArgsConfig(options, argConfig{name: "overrideArgs", extraArgs: []string{"-child"}}),
		},
	}

	for _, testCase := range testCases {
		(&testCase.config).mergeIncludedConfig(testCase.includedConfig)
		assert.Equal(t, testCase.config, testCase.expected, "For config %v and includeConfig %v", testCase.config, testCase.includedConfig)
	}
}

func TestParseTerragruntConfigTerraformNoSource(t *testing.T) {
	t.Parallel()

	config := `
		  terraform {
		  }
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.RemoteState)
	assert.Nil(t, terragruntConfig.Dependencies)

	if assert.NotNil(t, terragruntConfig.Terraform) {
		assert.Empty(t, terragruntConfig.Terraform.Source)
	}
}

func TestParseTerragruntConfigTerraformWithSource(t *testing.T) {
	t.Parallel()

	config := `
		  terraform {
		    source = "foo"
		  }
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.RemoteState)
	assert.Nil(t, terragruntConfig.Dependencies)

	if assert.NotNil(t, terragruntConfig.Terraform) {
		assert.Equal(t, "foo", terragruntConfig.Terraform.Source)
	}
}

func TestParseTerragruntConfigTerraformWithExtraArguments(t *testing.T) {
	t.Parallel()

	config := `
		  terraform {
		    extra_arguments "secrets" {
		      arguments = [
		        "-var-file=terraform-secret.tfvars"
		      ]
		      commands = get_terraform_commands_that_need_vars()
		    }
		  }
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.RemoteState)
	assert.Nil(t, terragruntConfig.Dependencies)

	assert.Equal(t, "secrets", terragruntConfig.ExtraArgs[0].Name)
	assert.Equal(t,
		[]string{
			"-var-file=terraform-secret.tfvars",
		},
		terragruntConfig.ExtraArgs[0].Arguments)
	assert.Equal(t,
		TerraformCommandWithVarFile,
		terragruntConfig.ExtraArgs[0].Commands)
}

func TestParseTerragruntConfigTerraformWithMultipleExtraArguments(t *testing.T) {
	t.Parallel()

	config := `
		  terraform {
		    extra_arguments "json_output" {
		      arguments = [
		        "-json"
		      ]
		      commands = [
		        "output"
		      ]
		    }

		    extra_arguments "fmt_diff" {
		      arguments = [
		        "-diff=true"
		      ]
		      commands = [
		        "fmt"
		      ]
		    }

		    extra_arguments "required_tfvars" {
		      required_var_files = [
		        "file1.tfvars",
						"file2.tfvars"
		      ]
		      commands = get_terraform_commands_that_need_vars()
		    }

		    extra_arguments "optional_tfvars" {
		      optional_var_files = [
		        "opt1.tfvars",
						"opt2.tfvars"
		      ]
		      commands = get_terraform_commands_that_need_vars()
		    }
		  }
	`

	terragruntConfig, err := parseConfigString(config, mockOptions, mockDefaultInclude)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, terragruntConfig.RemoteState)
	assert.Nil(t, terragruntConfig.Dependencies)
	assert.Equal(t, "json_output", terragruntConfig.ExtraArgs[0].Name)
	assert.Equal(t, []string{"-json"}, terragruntConfig.ExtraArgs[0].Arguments)
	assert.Equal(t, []string{"output"}, terragruntConfig.ExtraArgs[0].Commands)
	assert.Equal(t, "fmt_diff", terragruntConfig.ExtraArgs[1].Name)
	assert.Equal(t, []string{"-diff=true"}, terragruntConfig.ExtraArgs[1].Arguments)
	assert.Equal(t, []string{"fmt"}, terragruntConfig.ExtraArgs[1].Commands)
	assert.Equal(t, "required_tfvars", terragruntConfig.ExtraArgs[2].Name)
	assert.Equal(t, []string{"file1.tfvars", "file2.tfvars"}, terragruntConfig.ExtraArgs[2].RequiredVarFiles)
	assert.Equal(t, TerraformCommandWithVarFile, terragruntConfig.ExtraArgs[2].Commands)
	assert.Equal(t, "optional_tfvars", terragruntConfig.ExtraArgs[3].Name)
	assert.Equal(t, []string{"opt1.tfvars", "opt2.tfvars"}, terragruntConfig.ExtraArgs[3].OptionalVarFiles)
	assert.Equal(t, TerraformCommandWithVarFile, terragruntConfig.ExtraArgs[3].Commands)
}

func TestFindConfigFilesInPathOneNewConfig(t *testing.T) {
	t.Parallel()

	expected := []string{"../test/fixture-config-files/one-new-config/subdir/terragrunt.hcl"}
	actual, err := newOptionsWorkingDir("../test/fixture-config-files/one-new-config").FindConfigFilesInPath("")

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

func TestFindConfigFilesInPathMultipleConfigs(t *testing.T) {
	t.Parallel()

	expected := []string{
		"../test/fixture-config-files/multiple-configs/terragrunt.hcl",
		"../test/fixture-config-files/multiple-configs/subdir-3/terragrunt.hcl",
	}
	actual, err := newOptionsWorkingDir("../test/fixture-config-files/multiple-configs").FindConfigFilesInPath("")

	assert.Nil(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, actual)
}

func newOptionsWorkingDir(workingDir string) *options.TerragruntOptions {
	opts := options.NewTerragruntOptionsForTest(DefaultConfigName)
	opts.WorkingDir = workingDir
	return opts
}

func newOptionsVariables(variables map[string]interface{}) *options.TerragruntOptions {
	opts := options.NewTerragruntOptionsForTest("")
	newMap := make(map[string]options.Variable, len(variables))
	for key, value := range variables {
		newMap[key] = options.Variable{Value: value}
	}
	opts.Variables = newMap
	return opts
}
