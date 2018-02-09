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

	cmd = NewTFCmd(terragruntOptions).Args("not-a-real-command").Run()
	assert.Error(t, cmd)
}
