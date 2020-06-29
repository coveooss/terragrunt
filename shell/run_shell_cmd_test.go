package shell

import (
	"testing"

	"github.com/coveooss/terragrunt/v2/options"
	"github.com/stretchr/testify/assert"
)

func TestRunShellCommand(t *testing.T) {
	t.Parallel()

	terragruntOptions := options.NewTerragruntOptionsForTest("")
	cmd := NewTFCmd(terragruntOptions).Args("--version").Run()
	assert.Nil(t, cmd)

	value, err := NewTFCmd(terragruntOptions).Args("not-a-real-command").Output()
	assert.Error(t, err)
	assert.Contains(t, value, "Usage: terraform")
}
