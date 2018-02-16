package cli

import (
	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
)

func importDefaultVariables(terragruntOptions *options.TerragruntOptions, folder string) error {
	// Retrieve the default variables from the terraform files
	variables, err := util.LoadDefaultValues(folder)
	if err != nil {
		return err
	}
	for key, value := range variables {
		terragruntOptions.SetVariable(key, value, options.Default)
	}
	return nil
}

func setRoleEnvironmentVariables(terragruntOptions *options.TerragruntOptions, roleArn string) error {
	roleVars, err := aws_helper.AssumeRoleEnvironmentVariables(roleArn, "terragrunt")
	if err != nil {
		return err
	}

	for key, value := range roleVars {
		terragruntOptions.Env[key] = value
	}
	return nil
}
