package config

import (
	"fmt"
	"runtime"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	logging "github.com/op/go-logging"
)

// TerragruntExtensioner defines the interface that must be implemented by Terragrunt Extension objects
type TerragruntExtensioner interface {
	config() *TerragruntConfig
	description() string
	enabled() bool
	extraInfo() string
	help() string
	id() string
	init(*TerragruntConfig)
	logger() *logging.Logger
	name() string
	normalize()
	options() *options.TerragruntOptions
	run(args ...interface{}) ([]interface{}, error)
}

// TerragruntExtensionBase is the base object to define object used to extend the behavior of terragrunt
type TerragruntExtensionBase struct {
	_config *TerragruntConfig

	Name        string   `hcl:",key"`
	DisplayName string   `hcl:"display_name"`
	Description string   `hcl:"description"`
	OS          []string `hcl:"os"`
	Disabled    bool     `hcl:"disabled"`
}

func (base TerragruntExtensionBase) String() string      { return base.id() }
func (base TerragruntExtensionBase) id() string          { return base.Name }
func (base TerragruntExtensionBase) description() string { return base.Description }
func (base TerragruntExtensionBase) extraInfo() string   { return "" }
func (base TerragruntExtensionBase) normalize()          {}

func (base *TerragruntExtensionBase) init(config *TerragruntConfig) {
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
func (base TerragruntExtensionBase) config() *TerragruntConfig {
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
func (base TerragruntExtensionBase) logger() *logging.Logger {
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
