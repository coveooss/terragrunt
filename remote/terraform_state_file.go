package remote

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
)

// TODO: this file could be changed to use the Terraform Go code to read state files, but that code is relatively
// complicated and doesn't seem to be designed for standalone use. Fortunately, the .tfstate format is a fairly simple
// JSON format, so hopefully this simple parsing code will not be a maintenance burden.

// When storing Terraform state locally, this is the default path to the tfstate file
const defaultPathToLocalStateFile = "terraform.tfstate"

// When using remote state storage, Terraform keeps a local copy of the state file in this folder
const defaultPathToRemoteStateFile = ".terraform/terraform.tfstate"

// TerraformState is the structure representing the Terraform .tfstate file
type TerraformState struct {
	Version int
	Serial  int
	Backend *terraformBackend
	Modules []terraformStateModule
}

// The structure of the "backend" section of the Terraform .tfstate file
type terraformBackend struct {
	Type   string
	Config map[string]interface{}
}

// The structure of a "module" section of the Terraform .tfstate file
type terraformStateModule struct {
	Path      []string
	Outputs   map[string]interface{}
	Resources map[string]interface{}
}

// Return true if this Terraform state is configured for remote state storage
func (state *TerraformState) isRemote() bool {
	return state.Backend != nil && state.Backend.Type != "local"
}

// Parse the Terraform .tfstate file from the location specified by workingDir. If no location is specified,
// search the current directory. If the file doesn't exist at any of the default locations, return nil.
func parseTerraformStateFileFromLocation(workingDir string) (*TerraformState, error) {
	if util.FileExists(util.JoinPath(workingDir, defaultPathToLocalStateFile)) {
		return parseTerraformStateFile(util.JoinPath(workingDir, defaultPathToLocalStateFile))
	} else if util.FileExists(util.JoinPath(workingDir, defaultPathToRemoteStateFile)) {
		return parseTerraformStateFile(util.JoinPath(workingDir, defaultPathToRemoteStateFile))
	} else {
		return nil, nil
	}
}

// Parse the Terraform .tfstate file at the given path
func parseTerraformStateFile(path string) (*TerraformState, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, tgerrors.WithStackTrace(errCantParseTerraformStateFile{Path: path, UnderlyingErr: err})
	}

	return parseTerraformState(bytes)
}

// Parse the Terraform state file data in the given byte slice
func parseTerraformState(terraformStateData []byte) (*TerraformState, error) {
	terraformState := &TerraformState{}

	if err := json.Unmarshal(terraformStateData, terraformState); err != nil {
		return nil, tgerrors.WithStackTrace(err)
	}

	return terraformState, nil
}

type errCantParseTerraformStateFile struct {
	Path          string
	UnderlyingErr error
}

func (err errCantParseTerraformStateFile) Error() string {
	return fmt.Sprintf("Error parsing Terraform state file %s: %s", err.Path, err.UnderlyingErr.Error())
}
