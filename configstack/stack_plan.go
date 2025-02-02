package configstack

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/coveooss/terragrunt/v2/tgerrors"
	"github.com/coveooss/terragrunt/v2/util"
)

type planSummary struct {
	Message         string
	NumberOfChanges int
	AreChangesKnown bool
}

// The returned information for each module
type moduleResult struct {
	Module      TerraformModule
	Err         error
	PlanSummary planSummary
}

var planResultRegex = regexp.MustCompile(`(\d+) to add, (\d+) to change, (\d+) to destroy.`)

func (stack *Stack) planWithSummary(terragruntOptions *options.TerragruntOptions) error {
	// We override the multi errors creator to use a specialized error type for plan
	// because error severity in plan is not standard (i.e. exit code 2 is less significant that exit code 1).
	CreateMultiErrors = func(errs []error) error {
		return planMultiError{errMulti{errs}}
	}

	detailedExitCode := util.ListContainsElement(terragruntOptions.TerraformCliArgs, "-detailed-exitcode")

	hasChanges := false
	results := make([]moduleResult, 0, len(stack.Modules))
	err := runModulesWithHandler(stack.Modules, getResultHandler(detailedExitCode, &results, &hasChanges), NormalOrder)
	printSummary(terragruntOptions, results)

	// If there is no error, but -detail-exitcode is specified, we return an error with the number of changes.
	if err == nil && detailedExitCode {
		knownChanges := 0
		modulesWithUnknownChanges := 0
		for _, status := range results {
			if status.PlanSummary.AreChangesKnown {
				knownChanges += status.PlanSummary.NumberOfChanges
			} else {
				modulesWithUnknownChanges += 1
			}
		}

		if knownChanges != 0 {
			article, plural := "is", ""
			if knownChanges > 1 {
				article, plural = "are", "s"
			}
			terragruntOptions.Logger.Infof("There %s %v change%s to apply", article, knownChanges, plural)
		} else if hasChanges {
			terragruntOptions.Logger.Infof("There are no terraform changes but hooks have reported changes.")
		}

		if modulesWithUnknownChanges > 0 {
			plural := ""
			if modulesWithUnknownChanges > 1 {
				plural = "s"
			}

			terragruntOptions.Logger.Infof(
				"We were not able to determine the number of changes for %v module%s.",
				modulesWithUnknownChanges,
				plural,
			)
		}

		if hasChanges {
			return tgerrors.PlanWithChanges{}
		}
	}

	return err
}

// Returns the handler that will be executed after each completion of `terraform plan`
func getResultHandler(detailedExitCode bool, results *[]moduleResult, hasChanges *bool) ModuleHandler {
	return func(module TerraformModule, output string, err error) (string, error) {
		warnAboutMissingDependencies(module, output)
		if exitCode, convErr := shell.GetExitCode(err); convErr == nil && detailedExitCode && exitCode == tgerrors.ChangeExitCode {
			// We do not want to consider ChangeExitCode as an error and not execute the dependants because there is an "error" in the dependencies.
			// ChangeExitCode is not an error in this case, it is simply a status. We will reintroduce the exit code at the very end to mimic the behaviour
			// of the native terraform plan -detailed-exitcode to exit with ChangeExitCode if there are changes in any of the module in the stack.
			*hasChanges = true
			err = nil
		}

		if output != "" {
			summary := extractSummaryResultFromPlan(output)

			// We add the result to the result list (there is no concurrency problem because it is handled by the running_module)
			*results = append(*results, moduleResult{module, err, summary})
		}

		return output, err
	}
}

// Print a little summary of the plan execution
func printSummary(terragruntOptions *options.TerragruntOptions, results []moduleResult) {
	terragruntOptions.Printf("%s\nSummary:\n", separator)

	var length int
	for _, result := range results {
		nameLength := len(util.GetPathRelativeToWorkingDir(result.Module.Path))
		if nameLength > length {
			length = nameLength
		}
	}

	format := fmt.Sprintf("    %%-%dv : %%v%%v\n", length)
	for _, result := range results {
		errMsg := ""
		if result.Err != nil {
			errMsg = fmt.Sprintf(", Error: %v", result.Err)
		}

		terragruntOptions.Printf(
			format,
			util.GetPathRelativeToWorkingDir(result.Module.Path),
			result.PlanSummary.Message,
			errMsg,
		)
	}
}

// Check the output message
func warnAboutMissingDependencies(module TerraformModule, output string) {
	if strings.Contains(output, ": Resource 'data.terraform_remote_state.") {
		var dependenciesMsg string
		if len(module.Dependencies) > 0 {
			dependenciesMsg = fmt.Sprintf(" contains dependencies to %v and", module.Config.Dependencies.Paths)
		}
		module.TerragruntOptions.Logger.Warningf("%v%v refers to remote state, you may have to apply your changes in the dependencies prior running terragrunt plan-all.\n",
			module.Path,
			dependenciesMsg,
		)
	}
}

// Parse the output message to extract a summary
func extractSummaryResultFromPlan(output string) planSummary {
	noChanges := []string{
		"Plan: 0 to add, 0 to change, 0 to destroy.", // This was the message returned by terraform 0.11
		"No changes. Infrastructure is up-to-date.",  // This was the message returned by terraform 0.12
		"Your infrastructure matches the configuration.",
		"without changing any real infrastructure.",
	}
	for _, noChange := range noChanges {
		if strings.Contains(output, noChange) {
			return planSummary{"No change", 0, true}
		}
	}

	result := planResultRegex.FindStringSubmatch(output)
	if len(result) == 0 {
		return planSummary{"Unable to determine the plan status", -1, false}
	}

	// Count the total number of changes
	sum := 0
	for _, value := range result[1:] {
		count, _ := strconv.Atoi(value)
		sum += count
	}
	if sum != 0 {
		return planSummary{result[0], sum, true}
	}

	// Sometimes, terraform returns 0 add, 0 change and 0 destroy. We return a more explicit message
	return planSummary{"No effective change", 0, true}
}

// planMultiError is a specialized version of errMulti type
// It handles the exit code differently from the base implementation
type planMultiError struct {
	errMulti
}

// ExitStatus returns the numeric status corresponding with the list of errors.
func (e planMultiError) ExitStatus() (int, error) {
	exitCode := normalExitCode
	for i := range e.Errors {
		if code, err := shell.GetExitCode(e.Errors[i]); err != nil {
			return undefinedExitCode, e
		} else if code == errorExitCode || code == tgerrors.ChangeExitCode && exitCode == normalExitCode {
			// The exit code 1 is more significant that the exit code 2 because it represents an error
			// while 2 represent a warning.
			return undefinedExitCode, e
		}
	}
	return exitCode, nil
}
