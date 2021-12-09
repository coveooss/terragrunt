package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/multilogger/errors"
)

type extensionList []terragruntExtensioner

type extensionFilter func(terragruntExtensioner) bool

func extensionEnabled(ext terragruntExtensioner) bool { return ext.enabled() }

type extensionListCompatible interface {
	toGeneric(...extensionFilter) extensionList
	init(*TerragruntConfigFile) error
	merge(extensionList)
}

type extensionListSortable interface{ sort() }

func (list extensionList) init(config *TerragruntConfigFile) error {
	var errArray errors.Array
	for _, item := range list {
		if err := item.init(config, item); err != nil {
			errArray = append(errArray, err)
		}
	}
	return errArray.AsError()
}

func filterExtensions(list interface{}, filters []extensionFilter) extensionList {
	iteratable := reflect.ValueOf(list)
	result := make(extensionList, 0, iteratable.Len())
loop:
	for i := 0; i < iteratable.Len(); i++ {
		item := iteratable.Index(i).Interface().(terragruntExtensioner)
		for _, filter := range filters {
			if !filter(item) {
				continue loop
			}
		}
		result = append(result, item)
	}
	return result
}

func convert(list, target interface{}) interface{} {
	iteratable := reflect.ValueOf(list)
	targetType := reflect.TypeOf(target)
	if l := iteratable.Len(); l > 0 {
		copy := reflect.MakeSlice(targetType, l, l)
		for i := 0; i < l; i++ {
			copy.Index(i).Set(reflect.ValueOf(iteratable.Index(i).Interface()))
		}
		return copy.Interface()
	}
	return reflect.Zero(targetType).Interface()
}

func merge(list []terragruntExtensioner, imported []terragruntExtensioner, mode mergeMode) (result []terragruntExtensioner) {
	if len(imported) == 0 {
		return list
	} else if len(list) == 0 {
		return imported
	}

	if _, isIdentified := imported[0].(extensionerIdentified); isIdentified {
		log := imported[0].logger()
		getID := func(item terragruntExtensioner) string { return item.(extensionerIdentified).id() }
		// Create a map with existing elements
		index := make(map[string]int, len(list))
		for i, item := range list {
			index[getID(item)] = i
		}

		// Check if there are duplicated elements in the imported list
		indexImported := make(map[string]int, len(list))
		for i, item := range imported {
			indexImported[getID(item)] = i
		}

		// Create a list of the elements that should be added to the list
		newList := make([]terragruntExtensioner, 0, len(imported))
		for i, item := range imported {
			name := getID(item)
			if pos := indexImported[name]; pos != i {
				log.Warningf("Skipping previous definition of %s %v as it is overridden in the same file", item.itemType(), name)
				continue
			}
			if pos, exist := index[name]; exist {
				// It already exist in the list, so is is an override
				// We remove it from its current position and add it to the list of newly added elements to keep its original declaration ordering.
				newList = append(newList, list[pos])
				delete(index, name)
				log.Debugf("Skipping %s %v as it is overridden in the current config", item.itemType(), name)
				continue
			}
			newList = append(newList, item)
		}
		imported = newList

		if len(list) == 0 {
			return imported
		}

		if len(index) != len(list) {
			// Some elements must be removed from the original list, we simply regenerate the list
			// including only elements that are still in the index.
			cleanedList := make([]terragruntExtensioner, 0, len(index))
			for _, item := range list {
				name := getID(item)
				if _, found := index[name]; found {
					cleanedList = append(cleanedList, item)
				}
			}
			list = cleanedList
		}
	}

	if mode == mergeModeAppend {
		return append(list, imported...)
	}
	return append(imported, list...)
}

func help(list extensionListCompatible, listOnly bool, lookups ...string) (result string) {
	if sortable, ok := list.(extensionListSortable); ok {
		sortable.sort()
	}
	items := list.toGeneric()
	if len(items) == 0 {
		return
	}

	getID := func(item terragruntExtensioner) string { return item.name() }
	if _, isIdentified := items[0].(extensionerIdentified); isIdentified {
		getID = func(item terragruntExtensioner) string { return item.(extensionerIdentified).id() }
	}

	add := func(item terragruntExtensioner, name string) {
		extra := item.extraInfo()
		if extra != "" {
			extra = " " + extra
		}
		specificHelp := strings.TrimSpace(item.help())
		if specificHelp != "" {
			specificHelp = fmt.Sprintf("\n%s\n", collections.String(specificHelp).IndentN(2))
		}
		result += fmt.Sprintf("\n%s%s%s\n%s", TitleID(getID(item)), name, extra, specificHelp)
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
		add = func(item terragruntExtensioner, name string) {
			addLine(TitleID(getID(item)), name, item.extraInfo())
		}
	}

	for _, item := range items {
		match := len(lookups) == 0
		for i := 0; !match && i < len(lookups); i++ {
			match = strings.Contains(item.name(), lookups[i]) || strings.Contains(getID(item), lookups[i]) || strings.Contains(item.extraInfo(), lookups[i])
		}
		if !match {
			continue
		}
		var name string
		if getID(item) != item.name() {
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
