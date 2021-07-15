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

// Defines the interface that must be implemented by Terragrunt Extension objects
type terragruntExtensioner interface {
	config() *TerragruntConfigFile
	description() string
	enabled() bool
	extraInfo() string
	help() string
	init(*TerragruntConfigFile, terragruntExtensioner) error
	logger() *multilogger.Logger
	itemType() string
	name() string
	normalize() error // Used to assign default values
	options() *options.TerragruntOptions
}

type extensionerIdentified interface {
	id() string // Optional. Returns the id of the command (identified extensions can be overridden)
}

type extensionerOnCommand interface {
	onCommand() []string // Optional. Returns the list of commands on which this extension applies
}

type extensionerHelpDetails interface {
	helpDetails() string // Optional. Returns more detailled help context for an extension
}

// TerragruntExtension is the base object to define object used to extend the behavior of terragrunt
type TerragruntExtension struct {
	i       terragruntExtensioner
	_config *TerragruntConfigFile

	DisplayName string   `hcl:"display_name,optional"`
	Description string   `hcl:"description,optional"`
	OS          []string `hcl:"os,optional"`
	Disabled    bool     `hcl:"disabled,optional"`
}

// TerragruntExtensionIdentified is the base object to implement an overridable extension
type TerragruntExtensionIdentified struct {
	TerragruntExtension `hcl:",squash"`
	Name                string `hcl:"name,label"`
}

func (base *TerragruntExtensionIdentified) id() string { return base.Name }

func (base *TerragruntExtension) description() string { return base.Description }
func (base *TerragruntExtension) itemType() string    { panic("Not implemented") }
func (base *TerragruntExtension) extraInfo() string   { return "" }
func (base *TerragruntExtension) normalize() error    { return nil }

func (base *TerragruntExtension) String() string {
	if base.i == nil {
		return "Not initialized"
	}
	if identified, ok := base.i.(extensionerIdentified); ok {
		return identified.id()
	}
	return base.name()
}

func (base *TerragruntExtension) help() (result string) {
	if base.Description != "" {
		result += strings.TrimSpace(base.Description) + "\n"
	}

	if details, ok := base.i.(extensionerHelpDetails); ok {
		if details := strings.TrimSpace(details.helpDetails()); details != "" {
			result += details + "\n"
		}
	}
	if i, ok := base.i.(extensionerOnCommand); ok {
		result += fmt.Sprintf("\nApplies on the following command(s): %s\n", strings.Join(i.onCommand(), ", "))
	}
	if base.OS != nil {
		result += fmt.Sprintf("\nApplied only on the following OS: %s\n", strings.Join(base.OS, ", "))
	}
	return
}

func (base *TerragruntExtension) init(config *TerragruntConfigFile, i terragruntExtensioner) error {
	base.i = i
	base._config = config
	return i.normalize()
}

func (base *TerragruntExtension) name() string {
	if base.DisplayName != "" {
		return base.DisplayName
	}
	if identified, ok := base.i.(extensionerIdentified); ok {
		return identified.id()
	}
	if base.i == nil {
		return "Uninitialized"
	}
	return fmt.Sprintf("Unidentified %s", base.i.itemType())
}

// Config returns the current config associated with the object
func (base *TerragruntExtension) config() *TerragruntConfigFile {
	if base._config != nil {
		return base._config
	}
	panic(fmt.Sprintf("No config associated with object %v", base))
}

// Returns the current options set associated with the object
func (base *TerragruntExtension) options() *options.TerragruntOptions {
	if options := base.config().options; options != nil {
		return options
	}
	panic(fmt.Sprintf("No options set associated with object %v", base))
}

// Returns the current logger to use on the object
func (base *TerragruntExtension) logger() *multilogger.Logger {
	if logger := base.options().Logger; logger != nil {
		return logger
	}
	panic(fmt.Sprintf("No logger associated with object %v", base))
}

// Determines if a command is enabled
func (base *TerragruntExtension) enabled() bool {
	return !base.Disabled && (len(base.OS) == 0 || util.ListContainsElement(base.OS, runtime.GOOS))
}

// TitleID add formating to the id of the elements
var TitleID = color.New(color.FgHiYellow).SprintFunc()

type mergeMode int

const (
	mergeModePrepend mergeMode = iota
	mergeModeAppend
)
