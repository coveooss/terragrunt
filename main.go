package main

import (
	"os"

	"github.com/coveooss/multilogger"
	"github.com/coveooss/terragrunt/v2/cli"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/shell"
	"github.com/coveooss/terragrunt/v2/tgerrors"
)

// VERSION is set at build time using -ldflags parameters. For more info, see http://stackoverflow.com/a/11355611/483528
var VERSION = "2.7.13-local"

// The main entrypoint for Terragrunt
func main() {
	defer tgerrors.Recover(checkForErrorsAndExit)

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

		if _, ok := tgerrors.Unwrap(err).(tgerrors.PlanWithChanges); !ok {
			// Plan status are not considred as an error
			if os.Getenv(options.EnvDebug) != "" {
				logger.Error(tgerrors.PrintErrorWithStackTrace(err))
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
