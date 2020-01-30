package cli

import (
	"github.com/gruntwork-io/terragrunt/awshelper"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/terraform/configs"
)

func importDefaultVariables(terragruntOptions *options.TerragruntOptions, folder string) (map[string]*configs.Variable, error) {
	// Retrieve the default variables from the terraform files
	importedVariables, allVariables, err := util.LoadDefaultValues(folder)
	if err != nil {
		return allVariables, err
	}
	for key, value := range importedVariables {
		terragruntOptions.SetVariable(key, value, options.Default)
	}
	return allVariables, nil
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
