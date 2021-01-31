// +build genny
// lint:file-ignore U1000 Ignore all unused code, it's generated

package config

import (
	"github.com/cheekybits/genny/generic"
)

type Type generic.Type

// TypeList represents an array of Type
type TypeList []*Type

func (list TypeList) init(config *TerragruntConfigFile) { list.toGeneric().init(config) }

func (list *TypeList) merge(imported TypeList, mode mergeMode, argName string) {
	*list = toTypeList(merge(list.toGeneric(), imported.toGeneric(), mode, argName))
}

func (list TypeList) toGeneric(filters ...extensionFilter) extensionList {
	return filterExtensions(list, filters)
}

func toTypeList(list []terragruntExtensioner) TypeList {
	return convert(list, TypeList{}).(TypeList)
}

// Help returns the information relative to the elements within the list
func (list TypeList) Help(listOnly bool, lookups ...string) string {
	return help(list.Enabled(), listOnly, lookups...)
}

// Enabled returns only the enabled items on the list
func (list TypeList) Enabled() TypeList {
	return toTypeList(list.toGeneric(extensionEnabled))
}
