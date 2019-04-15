// This file was automatically generated by genny.
// Any changes will be lost if this file is regenerated.
// see https://github.com/cheekybits/genny

package config

import (
	"fmt"
	"strings"
)

// ApprovalConfigList represents an array of ApprovalConfig
type ApprovalConfigList []ApprovalConfig

// IApprovalConfig returns TerragruntExtensioner from the supplied type
func IApprovalConfig(item interface{}) TerragruntExtensioner {
	return item.(TerragruntExtensioner)
}

func (list ApprovalConfigList) init(config *TerragruntConfigFile) {
	for i := range list {
		IApprovalConfig(&list[i]).init(config)
	}
}

// Merge elements from an imported list to the current list priorising those already existing
func (list *ApprovalConfigList) merge(imported ApprovalConfigList, mode mergeMode, argName string) {
	if len(imported) == 0 {
		return
	}

	log := IApprovalConfig(&(imported)[0]).logger()

	// Create a map with existing elements
	index := make(map[string]int, len(*list))
	for i, item := range *list {
		index[IApprovalConfig(&item).id()] = i
	}

	// Check if there are duplicated elements in the imported list
	indexImported := make(map[string]int, len(*list))
	for i, item := range imported {
		indexImported[IImportVariables(&item).id()] = i
	}

	// Create a list of the hooks that should be added to the list
	newList := make(ApprovalConfigList, 0, len(imported))
	for i, item := range imported {
		name := IApprovalConfig(&item).id()
		if pos := indexImported[name]; pos != i {
			log.Warningf("Skipping previous definition of %s %v as it is overridden in the same file", argName, name)
			continue
		}
		if pos, exist := index[name]; exist {
			// It already exist in the list, so is is an override
			// We remove it from its current position and add it to the list of newly added elements to keep its original declaration ordering.
			newList = append(newList, (*list)[pos])
			delete(index, name)
			log.Infof("Skipping %s %v as it is overridden in the current config", argName, name)
			continue
		}
		newList = append(newList, item)
	}

	if len(*list) == 0 {
		*list = newList
		return
	}

	if len(index) != len(*list) {
		// Some elements must be removed from the original list, we simply regenerate the list
		// including only elements that are still in the index.
		newList := make(ApprovalConfigList, 0, len(index))
		for _, item := range *list {
			name := IApprovalConfig(&item).id()
			if _, found := index[name]; found {
				newList = append(newList, item)
			}
		}
		*list = newList
	}

	if mode == mergeModeAppend {
		*list = append(*list, newList...)
	} else {
		*list = append(newList, *list...)
	}
}

// Help returns the information relative to the elements within the list
func (list ApprovalConfigList) Help(listOnly bool, lookups ...string) (result string) {
	list.sort()
	add := func(item TerragruntExtensioner, name string) {
		extra := item.extraInfo()
		if extra != "" {
			extra = " " + extra
		}
		result += fmt.Sprintf("\n%s%s%s\n%s", TitleID(item.id()), name, extra, item.help())
	}

	var table [][]string
	width := []int{30, 0, 0}

	if listOnly {
		addLine := func(values ...string) {
			table = append(table, values)
			for i, value := range values {
				if len(value) > width[i] {
					width[i] = len(value)
				}
			}
		}
		add = func(item TerragruntExtensioner, name string) {
			addLine(TitleID(item.id()), name, item.extraInfo())
		}
	}

	for _, item := range list.Enabled() {
		item := IApprovalConfig(&item)
		match := len(lookups) == 0
		for i := 0; !match && i < len(lookups); i++ {
			match = strings.Contains(item.name(), lookups[i]) || strings.Contains(item.id(), lookups[i]) || strings.Contains(item.extraInfo(), lookups[i])
		}
		if !match {
			continue
		}
		var name string
		if item.id() != item.name() {
			name = " " + item.name()
		}
		add(item, name)
	}

	if listOnly {
		for i := range table {
			result += fmt.Sprintln()
			for j := range table[i] {
				result += fmt.Sprintf("%-*s", width[j]+1, table[i][j])
			}
		}
	}

	return
}

// Enabled returns only the enabled items on the list
func (list ApprovalConfigList) Enabled() ApprovalConfigList {
	result := make(ApprovalConfigList, 0, len(list))
	for _, item := range list {
		iItem := IApprovalConfig(&item)
		if iItem.enabled() {
			iItem.normalize()
			result = append(result, item)
		}
	}
	return result
}
