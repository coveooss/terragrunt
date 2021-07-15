// This file was automatically generated by genny.
// Any changes will be lost if this file is regenerated.
// see https://github.com/cheekybits/genny

//lint:file-ignore U1000 Ignore all unused code, it's generated

package config

// ImportFilesList represents an array of ImportFiles
type ImportFilesList []*ImportFiles

func (list ImportFilesList) init(config *TerragruntConfigFile) error {
	return list.toGeneric().init(config)
}

func (list ImportFilesList) toGeneric(filters ...extensionFilter) extensionList {
	return filterExtensions(list, filters)
}

func (list *ImportFilesList) merge(data extensionList) {
	*list = toImportFilesList(merge(list.toGeneric(), data, list.mergeMode()))
}

func toImportFilesList(list []terragruntExtensioner) ImportFilesList {
	converted := convert(list, ImportFilesList{}).(ImportFilesList)
	return converted
}

func (item ImportFiles) itemType() string {
	return ImportFilesList{}.argName()
}

// Help returns the information relative to the elements within the list
func (list ImportFilesList) Help(listOnly bool, lookups ...string) string {
	enabled := list.Enabled()
	return help(&enabled, listOnly, lookups...)
}

// Enabled returns only the enabled items on the list
func (list ImportFilesList) Enabled() ImportFilesList {
	return toImportFilesList(list.toGeneric(extensionEnabled))
}

// Filter is used to filter the list on supplied criteria
func (list ImportFilesList) Filter(filter ImportFilesFilter) ImportFilesList {
	result := make(ImportFilesList, 0, len(list))
	for _, item := range list.Enabled() {
		if filter(item) {
			result = append(result, item)
		}
	}
	return result
}

// ImportFilesFilter describe a function able to filter ImportFiles from a list
type ImportFilesFilter func(*ImportFiles) bool
