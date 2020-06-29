package main

import (
	"os"

	"github.com/coveooss/multilogger"
	"github.com/coveooss/terragrunt/v2/cli"
	"github.com/coveooss/terragrunt/v2/errors"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
)

// VERSION is set at build time using -ldflags parameters. For more info, see http://stackoverflow.com/a/11355611/483528
var VERSION = "1.4.1"

// The main entrypoint for Terragrunt
func main() {
	defer errors.Recover(checkForErrorsAndExit)

	app := cli.CreateTerragruntCli(VERSION, os.Stdout, os.Stderr)
	err := app.Run(os.Args)

	checkForErrorsAndExit(err)
}

// If there is an error, display it in the console and exit with a non-zero exit code. Otherwise, exit 0.
func checkForErrorsAndExit(err error) {
	if err == nil {
		os.Exit(0)
	} else {
		logger := multilogger.New("terragrunt")

		if _, ok := errors.Unwrap(err).(errors.PlanWithChanges); !ok {
			// Plan status are not considred as an error
			if os.Getenv(options.EnvDebug) != "" {
				logger.Error(errors.PrintErrorWithStackTrace(err))
			} else {
				logger.Error(err)
			}
		}

		// exit with the underlying error code
		exitCode, exitCodeErr := shell.GetExitCode(err)
		if exitCodeErr != nil {
			exitCode = 1
			logger.Error("Unable to determine underlying exit code, so Terragrunt will exit with error code 1")
		}
		os.Exit(exitCode)
	}
}
