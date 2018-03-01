package shell

import (
	"testing"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/stretchr/testify/assert"
)

func TestRunShellCommand(t *testing.T) {
	t.Parallel()

	terragruntOptions := options.NewTerragruntOptionsForTest("")
	cmd := NewTFCmd(terragruntOptions).Args("--version").Run()
	assert.Nil(t, cmd)

	value, err := NewTFCmd(terragruntOptions).Args("not-a-real-command").Output()
	assert.Nil(t, err)
	assert.Contains(t, value, "Usage: terraform")
}
