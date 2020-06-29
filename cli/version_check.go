package cli

import (
	"fmt"
	"regexp"

	"github.com/coveooss/terragrunt/v2/errors"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/hashicorp/go-version"
)

// The terraform --version output is of the format: Terraform v0.9.5-dev (cad024a5fe131a546936674ef85445215bbc4226+CHANGES)
// where -dev and (committed+CHANGES) is for custom builds or if TF_LOG is set for debug purposes
var terraformVersionRegex = regexp.MustCompile(`Terraform (v?[\d\.]+)(?:-dev)?(?: .+)?`)

// CheckTerraformVersion checks that the currently installed Terraform version works meets the specified version constraint
// and returns an error if it doesn't
func CheckTerraformVersion(constraint string, terragruntOptions *options.TerragruntOptions) error {
	currentVersion, err := getTerraformVersion(terragruntOptions)
	if err != nil {
		return err
	}

	terraformVersion = currentVersion.String()
	return checkTerraformVersionMeetsConstraint(currentVersion, constraint)
}

// Check that the current version of Terraform meets the specified constraint and return an error if it doesn't
func checkTerraformVersionMeetsConstraint(currentVersion *version.Version, constraint string) error {
	versionConstraint, err := version.NewConstraint(constraint)
	if err != nil {
		return err
	}

	if !versionConstraint.Check(currentVersion) {
		return errors.WithStackTrace(ErrInvalidTerraformVersion{CurrentVersion: currentVersion, VersionConstraints: versionConstraint})
	}

	return nil
}

// Get the currently installed version of Terraform
func getTerraformVersion(terragruntOptions *options.TerragruntOptions) (*version.Version, error) {
	output, err := shell.NewTFCmd(terragruntOptions).Args("--version").Output()
	if err != nil {
		return nil, err
	}

	return parseTerraformVersion(output)
}

// Parse the output of the terraform --version command
func parseTerraformVersion(versionCommandOutput string) (*version.Version, error) {
	matches := terraformVersionRegex.FindStringSubmatch(versionCommandOutput)

	if len(matches) != 2 {
		return nil, errors.WithStackTrace(ErrInvalidTerraformVersionSyntax(versionCommandOutput))
	}

	return version.NewVersion(matches[1])
}

// Custom error types

// ErrInvalidTerraformVersionSyntax indicates that we cannot retrieve the terraform version
type ErrInvalidTerraformVersionSyntax string

func (err ErrInvalidTerraformVersionSyntax) Error() string {
	return fmt.Sprintf("Unable to parse Terraform version output: %s", string(err))
}

// ErrInvalidTerraformVersion indicates that the Terraform version is not compatible with this version of Terragrunt
type ErrInvalidTerraformVersion struct {
	CurrentVersion     *version.Version
	VersionConstraints version.Constraints
}

func (err ErrInvalidTerraformVersion) Error() string {
	return fmt.Sprintf("The currently installed version of Terraform (%s) is not compatible with the version Terragrunt requires (%s).", err.CurrentVersion.String(), err.VersionConstraints.String())
}
