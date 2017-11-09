package config

import (
	"fmt"
	"runtime"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	logging "github.com/op/go-logging"
)

// TerragruntExtensioner defines the interface that must be implemented by Terragrunt Extension objects
type TerragruntExtensioner interface {
	Id() interface{}
}

// TerragruntExtensionBase is the base object to define object used to extend the behavior of terragrunt
type TerragruntExtensionBase struct {
	Name        string `hcl:",key"`
	DisplayName string `hcl:"display_name"`
	Description string
	OS          []string

	config *TerragruntConfig
}

// GetName returns the name of the object
func (base *TerragruntExtensionBase) GetName() string {
	if base.DisplayName != "" {
		return base.DisplayName
	}
	return base.Name
}

// Config returns the current config associated with the object
func (base *TerragruntExtensionBase) Config() *TerragruntConfig {
	if base.config != nil {
		return base.config
	}
	panic(fmt.Sprintf("No config associated with object %v", base.GetName()))
}

// Options returns the current options set associated with the object
func (base *TerragruntExtensionBase) Options() *options.TerragruntOptions {
	if options := base.Config().options; options != nil {
		return options
	}
	panic(fmt.Sprintf("No options set associated with object %v", base.GetName()))
}

// Logger returns the current logger to use on the object
func (base *TerragruntExtensionBase) Logger() *logging.Logger {
	if logger := base.Options().Logger; logger != nil {
		return logger
	}
	panic(fmt.Sprintf("No logger associated with object %v", base.GetName()))
}

// Enabled determines if a command is enabled
func (base *TerragruntExtensionBase) Enabled() bool {
	return len(base.OS) == 0 || util.ListContainsElement(base.OS, runtime.GOOS)
}
