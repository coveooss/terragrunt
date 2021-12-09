// +build genny
//lint:file-ignore U1000 Ignore all unused code, it's generated

package config

import (
	"github.com/cheekybits/genny/generic"
)

type TypeName generic.Type

// TypeNameList represents an array of TypeName
type TypeNameList []*TypeName

func (list TypeNameList) init(config *TerragruntConfigFile) error {
	return list.toGeneric().init(config)
}

func (list TypeNameList) toGeneric(filters ...extensionFilter) extensionList {
	return filterExtensions(list, filters)
}

func (list *TypeNameList) merge(data extensionList) {
	*list = toTypeNameList(merge(list.toGeneric(), data, list.mergeMode()))
}

func toTypeNameList(list []terragruntExtensioner) TypeNameList {
	converted := convert(list, TypeNameList{}).(TypeNameList)
	return converted
}

func (item TypeName) itemType() string {
	return TypeNameList{}.argName()
}

// Help returns the information relative to the elements within the list
func (list TypeNameList) Help(listOnly bool, lookups ...string) string {
	enabled := list.Enabled()
	return help(&enabled, listOnly, lookups...)
}

// Enabled returns only the enabled items on the list
func (list TypeNameList) Enabled() TypeNameList {
	return toTypeNameList(list.toGeneric(extensionEnabled))
}

// Filter is used to filter the list on supplied criteria
func (list TypeNameList) Filter(filter TypeNameFilter) TypeNameList {
	result := make(TypeNameList, 0, len(list))
	for _, item := range list.Enabled() {
		if filter(item) {
			result = append(result, item)
		}
	}
	return result
}

// TypeNameFilter describe a function able to filter TypeName from a list
type TypeNameFilter func(*TypeName) bool
