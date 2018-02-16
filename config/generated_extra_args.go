// This file was automatically generated by genny.
// Any changes will be lost if this file is regenerated.
// see https://github.com/cheekybits/genny

package config

import (
	"fmt"
	"strings"
)

// TerraformExtraArgumentsList represents an array of TerraformExtraArguments
type TerraformExtraArgumentsList []TerraformExtraArguments

// ITerraformExtraArguments returns TerragruntExtensioner from the supplied type
func ITerraformExtraArguments(item interface{}) TerragruntExtensioner {
	return item.(TerragruntExtensioner)
}

func (list TerraformExtraArgumentsList) init(config *TerragruntConfigFile) {
	for i := range list {
		ITerraformExtraArguments(&list[i]).init(config)
	}
}

// Merge elements from an imported list to the current list priorising those already existing
func (list *TerraformExtraArgumentsList) merge(imported TerraformExtraArgumentsList, mode mergeMode, argName string) {
	if len(imported) == 0 {
		return
	} else if len(*list) == 0 {
		*list = imported
		return
	}

	log := ITerraformExtraArguments(&(*list)[0]).logger().Debugf

	// Create a map with existing elements
	index := make(map[string]int, len(*list))
	for i, item := range *list {
		index[ITerraformExtraArguments(&item).id()] = i
	}

	// Create a list of the hooks that should be added to the list
	newList := make(TerraformExtraArgumentsList, 0, len(imported))
	for _, item := range imported {
		name := ITerraformExtraArguments(&item).id()
		if pos, exist := index[name]; exist {
			// It already exist in the list, so is is an override
			// We remove it from its current position and add it to the list of newly added elements to keep its original declaration ordering.
			newList = append(newList, (*list)[pos])
			delete(index, name)
			log("Skipping %s %v as it is overridden in the current config", argName, name)
			continue
		}
		newList = append(newList, item)
	}

	if len(index) != len(*list) {
		// Some elements must be removed from the original list, we simply regenerate the list
		// including only elements that are still in the index.
		newList := make(TerraformExtraArgumentsList, 0, len(index))
		for _, item := range *list {
			name := ITerraformExtraArguments(&item).id()
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
func (list TerraformExtraArgumentsList) Help(listOnly bool, lookups ...string) (result string) {
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
		item := ITerraformExtraArguments(&item)
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
func (list TerraformExtraArgumentsList) Enabled() TerraformExtraArgumentsList {
	result := make(TerraformExtraArgumentsList, 0, len(list))
	for _, item := range list {
		iItem := ITerraformExtraArguments(&item)
		if iItem.enabled() {
			iItem.normalize()
			result = append(result, item)
		}
	}
	return result
}

// Run execute the content of the list
func (list TerraformExtraArgumentsList) Run(args ...interface{}) (result []interface{}, err error) {
	if len(list) == 0 {
		return
	}

	list.sort()

	for _, item := range list {
		iItem := ITerraformExtraArguments(&item)
		var temp interface{}
		iItem.logger().Infof("Running %s (%s): %s", iItem.itemType(), iItem.id(), iItem.name())
		iItem.normalize()
		if temp, err = iItem.run(args...); err != nil {
			err = fmt.Errorf("Error while executing %s(%s): %v", iItem.itemType(), iItem.id(), err)
			return
		}
		result = append(result, temp)
	}
	return
}
