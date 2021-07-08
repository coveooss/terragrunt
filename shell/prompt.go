package shell

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/tgerrors"
)

// Prompt the user for text in the CLI. Returns the text entered by the user.
func promptUserForInput(prompt string, terragruntOptions *options.TerragruntOptions) (string, error) {
	if terragruntOptions.Logger.GetModule() != "" {
		prompt = fmt.Sprintf("%s %s", terragruntOptions.Logger.GetModule(), prompt)
	}
	fmt.Print(prompt)

	if terragruntOptions.NonInteractive {
		terragruntOptions.Logger.Info("\nThe non-interactive flag is set to true, so assuming 'yes' for all prompts")
		return "yes", nil
	}

	reader := bufio.NewReader(os.Stdin)

	text, err := reader.ReadString('\n')
	if err != nil {
		return "", tgerrors.WithStackTrace(err)
	}

	return strings.TrimSpace(text), nil
}

// PromptUserForYesNo prompts the user for a yes/no response and return true if they entered yes.
func PromptUserForYesNo(prompt string, terragruntOptions *options.TerragruntOptions) (bool, error) {
	resp, err := promptUserForInput(fmt.Sprintf("%s (y/n) ", prompt), terragruntOptions)

	if err != nil {
		return false, tgerrors.WithStackTrace(err)
	}

	switch strings.ToLower(resp) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
