//go:build linux || darwin
// +build linux darwin

package test

import (
	"fmt"
	"testing"
)

func TestLocalWithRelativeExtraArgsUnix(t *testing.T) {
	t.Parallel()

	const testPath = "fixture-download/local-relative-extra-args-unix"
	cleanupTerraformFolder(t, testPath)

	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", testPath))

	// Run a second time to make sure the temporary folder can be reused without errors
	runTerragrunt(t, fmt.Sprintf("terragrunt apply --terragrunt-non-interactive --terragrunt-working-dir %s", testPath))
}
