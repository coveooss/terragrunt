package remote

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

// State is the configuration for Terraform remote state
type State struct {
	Backend string `hcl:"backend,optional"`
	Config  map[string]interface{}

	ConfigHclDefinition cty.Value `hcl:"config,optional"`
}

func (remoteState *State) String() string {
	return fmt.Sprintf("RemoteState{Backend = %v, Config = %v}", remoteState.Backend, remoteState.Config)
}

type remoteStateInitializer func(map[string]interface{}, *options.TerragruntOptions) error

// TODO: initialization actions for other remote state backends can be added here
var remoteStateInitializers = map[string]remoteStateInitializer{
	"s3": initializeRemoteStateS3,
}

// Validate that the remote state is configured correctly
func (remoteState *State) Validate() error {
	if !remoteState.ConfigHclDefinition.IsNull() {
		if err := util.FromCtyValue(remoteState.ConfigHclDefinition, &remoteState.Config); err != nil {
			return err
		}
	}

	if remoteState.Backend == "" {
		return errors.WithStackTrace(ErrBackendMissing)
	}
	return nil
}

// Initialize performs any actions necessary to initialize the remote state before it's used for storage. For example, if you're
// using S3 for remote state storage, this may create the S3 bucket if it doesn't exist already.
func (remoteState *State) Initialize(terragruntOptions *options.TerragruntOptions) error {
	initializer, hasInitializer := remoteStateInitializers[remoteState.Backend]
	if hasInitializer {
		return initializer(remoteState.Config, terragruntOptions)
	}

	return nil
}

// ConfigureRemoteState configures Terraform remote state
func (remoteState State) ConfigureRemoteState(terragruntOptions *options.TerragruntOptions) error {
	shouldConfigure, err := shouldConfigureRemoteState(remoteState, terragruntOptions)
	if err != nil {
		return err
	}

	if shouldConfigure {
		terragruntOptions.Logger.Debugf("Initializing remote state for the %s backend", remoteState.Backend)
		if err := remoteState.Initialize(terragruntOptions); err != nil {
			return err
		}

		terragruntOptions.Logger.Debugf("Configuring remote state for the %s backend", remoteState.Backend)
		return shell.NewTFCmd(terragruntOptions).Args(initCommand(remoteState)...).WithRetries(3).LogOutput(logrus.DebugLevel)
	}

	return nil
}

// Returns true if remote state needs to be configured. This will be the case when:
//
// 1. Remote state has not already been configured
// 2. Remote state has been configured, but for a different backend type, and the user confirms it's OK to overwrite it.
func shouldConfigureRemoteState(remoteStateFromTerragruntConfig State, terragruntOptions *options.TerragruntOptions) (bool, error) {
	state, err := parseTerraformStateFileFromLocation(terragruntOptions.WorkingDir)
	if err != nil {
		return false, err
	}

	if state != nil && state.isRemote() {
		return shouldOverrideExistingRemoteState(state.Backend, remoteStateFromTerragruntConfig, terragruntOptions)
	}
	return true, nil
}

// Check if the remote state that is already configured matches the one specified in the Terragrunt config. If it does,
// return false to indicate remote state does not need to be configured again. If it doesn't, prompt the user whether
// we should override the existing remote state setting.
func shouldOverrideExistingRemoteState(existingBackend *terraformBackend, remoteStateFromTerragruntConfig State, terragruntOptions *options.TerragruntOptions) (bool, error) {
	if existingBackend.Type != remoteStateFromTerragruntConfig.Backend {
		terragruntOptions.Logger.Warning("Terraform remote state is already configured for a different backend", existingBackend.Type)
		prompt := fmt.Sprintf("Current backend = %s\nNew backend = %s\n\nOverwrite?", existingBackend.Type, remoteStateFromTerragruntConfig.Backend)
		return shell.PromptUserForYesNo(prompt, terragruntOptions)
	}

	// Terraform's `backend` configuration uses a boolean for the `encrypt` parameter. However, perhaps for backwards compatibility reasons,
	// Terraform stores that parameter as a string in the `terraform.tfstate` file. Therefore, we have to convert it accordingly, or `DeepEqual`
	// will fail.
	if util.KindOf(existingBackend.Config["encrypt"]) == reflect.String && util.KindOf(remoteStateFromTerragruntConfig.Config["encrypt"]) == reflect.Bool {
		// If encrypt in remoteStateFromTerragruntConfig is a bool and a string in existingBackend, DeepEqual will consider the maps to be different.
		// So we convert the value from string to bool to make them equivalent.
		if value, err := strconv.ParseBool(existingBackend.Config["encrypt"].(string)); err == nil {
			existingBackend.Config["encrypt"] = value
		} else {
			terragruntOptions.Logger.Errorf("Remote state configuration encrypt contains invalid value %v, should be boolean.", existingBackend.Config["encrypt"])
		}
	}

	if !terraformStateConfigEqual(existingBackend.Config, remoteStateFromTerragruntConfig.Config) {
		getValues := func(config map[string]interface{}) string {
			result := make([]string, 0, len(config))
			for key := range config {
				result = append(result, key)
			}
			sort.Strings(result)
			for i, key := range result {
				result[i] = fmt.Sprint(key, "=", config[key])
			}
			return strings.Join(result, "\n\t")
		}

		terragruntOptions.Logger.Warning("Terraform remote state is already configured for backend", existingBackend.Type)
		prompt := fmt.Sprintf("\n    Existing config:\n\t%v\n\n    New config:\n\t%v\n\nOverwrite?", getValues(existingBackend.Config), getValues(remoteStateFromTerragruntConfig.Config))
		return shell.PromptUserForYesNo(prompt, terragruntOptions)
	}

	terragruntOptions.Logger.Debug("Remote state is already configured for backend", existingBackend.Type)
	return false, nil
}

func initCommand(remoteState State) []string {
	return append([]string{"init"}, remoteState.ToTerraformInitArgs()...)
}

// ToTerraformInitArgs converts the State config into the format used by the terraform init command
func (remoteState State) ToTerraformInitArgs() []string {
	backendConfigArgs := make([]string, 0, len(remoteState.Config))
	for key, value := range remoteState.Config {
		arg := fmt.Sprintf("-backend-config=%s=%v", key, value)
		backendConfigArgs = append(backendConfigArgs, arg)
	}

	backendConfigArgs = append(backendConfigArgs, "-force-copy", "-get=false")
	return backendConfigArgs
}

// ErrBackendMissing indicates that there is no backend configration defined.
var ErrBackendMissing = fmt.Errorf("the remote_state.backend field cannot be empty")

// Return true if the existing config from a .tfstate file is equal to the new config from the user's backend
// configuration. Under the hood, this method does a reflect.DeepEqual check, but with one twist: we strip out any
// null values in the existing config. This is because Terraform >= 0.12 stores ALL possible keys for a given backend
// in the .tfstate file, even if the user hasn't configured that key, in which case the value will be null, and cause
// reflect.DeepEqual to fail.
func terraformStateConfigEqual(existingConfig map[string]interface{}, newConfig map[string]interface{}) bool {
	if existingConfig == nil {
		return newConfig == nil
	}

	existingConfigNonNil := map[string]interface{}{}
	for existingKey, existingValue := range existingConfig {
		_, newValueIsSet := newConfig[existingKey]
		if existingValue == nil && !newValueIsSet {
			continue
		}
		existingConfigNonNil[existingKey] = existingValue
	}

	return reflect.DeepEqual(existingConfigNonNil, newConfig)
}
