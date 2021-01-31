package config

import (
	"fmt"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
)

// ApprovalConfig represents an `expect` format configuration that instructs terragrunt to wait for input on an ExpectStatement
// and to exit the command on a CompletedStatement
type ApprovalConfig struct {
	TerragruntExtensionBase `hcl:",remain"`

	Commands            []string `hcl:"commands"`
	ExpectStatements    []string `hcl:"expect_statements"`
	CompletedStatements []string `hcl:"completed_statements"`
}

func (item ApprovalConfig) itemType() (result string) { return ApprovalConfigList{}.argName() }
func (item ApprovalConfig) onCommand() []string       { return item.Commands }

func (item ApprovalConfig) helpDetails() string {
	result := "Runs the command"
	result += fmt.Sprintf("\nWaits for input, these statements: %s", strings.Join(item.ExpectStatements, ", "))
	result += "\nContinues the command execution"
	result += fmt.Sprintf("\nThen waits for completion, these statements: %s", strings.Join(item.CompletedStatements, ", "))
	return result
}

func (item ApprovalConfig) String() string {
	return collections.PrettyPrintStruct(item)
}

// ----------------------- ApprovalConfigList -----------------------

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.approval_config.go gen Type=ApprovalConfig
func (list ApprovalConfigList) argName() string { return "approval_config" }

// ShouldBeApproved looks for an approval config that corresponds to the given command. If if exists, it's returned with the value `true`.
func (list ApprovalConfigList) ShouldBeApproved(command string) (bool, *ApprovalConfig) {
	for _, approvalConfig := range list {
		for _, commandInConfig := range approvalConfig.Commands {
			if commandInConfig == command {
				return true, approvalConfig
			}
		}
	}
	return false, nil
}

// Merge elements from an imported list to the current list
func (list *ApprovalConfigList) Merge(imported ApprovalConfigList) {
	list.merge(imported, mergeModeAppend, "approval_config")
}
