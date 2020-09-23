package cli

import (
	"github.com/coveooss/terragrunt/v2/awshelper"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/util"
)

func importDefaultVariables(terragruntOptions *options.TerragruntOptions, folder string) error {
	// Retrieve the default variables from the terraform files
	importedVariables, _, err := util.LoadDefaultValues(folder, terragruntOptions.Logger, true)
	if err != nil {
		return err
	}
	for key, value := range importedVariables {
		terragruntOptions.SetVariable(key, value, options.Default)
	}
	return nil
}

func setRoleEnvironmentVariables(terragruntOptions *options.TerragruntOptions, roleArn string, assumeDuration *int) error {
	roleVars, err := awshelper.AssumeRoleEnvironmentVariables(terragruntOptions.Logger, roleArn, "terragrunt", assumeDuration)
	if err != nil {
		return err
	}

	for key, value := range roleVars {
		terragruntOptions.Env[key] = value
	}
	return nil
}
