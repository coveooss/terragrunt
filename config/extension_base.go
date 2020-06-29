//lint:file-ignore U1000 Ignore all unused code, it's generated

package config

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/coveooss/multilogger"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/fatih/color"
)

// TerragruntExtensioner defines the interface that must be implemented by Terragrunt Extension objects
type TerragruntExtensioner interface {
	config() *TerragruntConfigFile
	description() string
	enabled() bool
	extraInfo() string
	help() string
	id() string
	init(*TerragruntConfigFile)
	logger() *multilogger.Logger
	name() string
	itemType() string
	normalize() // Used to assign default values
	options() *options.TerragruntOptions
}

// TerragruntExtensionBase is the base object to define object used to extend the behavior of terragrunt
type TerragruntExtensionBase struct {
	_config *TerragruntConfigFile

	Name        string   `hcl:"name,label"`
	DisplayName string   `hcl:"display_name,optional"`
	Description string   `hcl:"description,optional"`
	OS          []string `hcl:"os,optional"`
	Disabled    bool     `hcl:"disabled,optional"`
}

func (base TerragruntExtensionBase) String() string      { return base.id() }
func (base TerragruntExtensionBase) id() string          { return base.Name }
func (base TerragruntExtensionBase) description() string { return base.Description }
func (base TerragruntExtensionBase) extraInfo() string   { return "" }
func (base TerragruntExtensionBase) normalize()          {}

func (base *TerragruntExtensionBase) init(config *TerragruntConfigFile) {
	base._config = config
}

func (base TerragruntExtensionBase) run(args ...interface{}) ([]interface{}, error) {
	return nil, nil
}

func (base TerragruntExtensionBase) name() string {
	if base.DisplayName != "" {
		return base.DisplayName
	}
	return base.Name
}

// Config returns the current config associated with the object
func (base TerragruntExtensionBase) config() *TerragruntConfigFile {
	if base._config != nil {
		return base._config
	}
	panic(fmt.Sprintf("No config associated with object %v", base.id()))
}

// Returns the current options set associated with the object
func (base TerragruntExtensionBase) options() *options.TerragruntOptions {
	if options := base.config().options; options != nil {
		return options
	}
	panic(fmt.Sprintf("No options set associated with object %v", base.id()))
}

// Returns the current logger to use on the object
func (base TerragruntExtensionBase) logger() *multilogger.Logger {
	if logger := base.options().Logger; logger != nil {
		return logger
	}
	panic(fmt.Sprintf("No logger associated with object %v", base.id()))
}

// Determines if a command is enabled
func (base TerragruntExtensionBase) enabled() bool {
	return !base.Disabled && (len(base.OS) == 0 || util.ListContainsElement(base.OS, runtime.GOOS))
}

// TitleID add formating to the id of the elements
var TitleID = color.New(color.FgHiYellow).SprintFunc()

type mergeMode int

const (
	mergeModePrepend mergeMode = iota
	mergeModeAppend
)

func (list GenericItemList) sort() GenericItemList { return list }

type errorArray []error

func (errors errorArray) Error() string {
	errorsStr := make([]string, 0, len(errors))
	for i := range errors {
		if errors[i] != nil {
			errorsStr = append(errorsStr, errors[i].Error())
		}
	}
	return strings.Join(errorsStr, "\n")
}
