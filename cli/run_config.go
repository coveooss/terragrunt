package cli

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/user"

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
	var userName string
	if userName = os.Getenv(options.EnvAssumedRoleID); userName == "" {
		if user, err := user.Current(); err != nil {
			userName = user.Username
		} else {
			if userName = os.Getenv("LOGNAME"); userName == "" {
				userName = os.Getenv("USER")
			}
		}
		if userName == "" {
			userName = "Unknown"
		}
	}
	uniqueID := rand.Intn(int(math.Pow(2, 24)))
	sessionName := fmt.Sprintf("terragrunt-%s-%06X", userName, uniqueID)

	roleVars, err := awshelper.AssumeRoleEnvironmentVariables(terragruntOptions.Logger, roleArn, sessionName, assumeDuration)
	if err != nil {
		return err
	}

	for key, value := range roleVars {
		terragruntOptions.Env[key] = value
	}
	return nil
}
