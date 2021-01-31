package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
)

type extensionList []terragruntExtensioner

type extensionFilter func(terragruntExtensioner) bool

func extensionEnabled(ext terragruntExtensioner) bool { return ext.enabled() }

type extensionListCompatible interface {
	toGeneric(...extensionFilter) extensionList
}

type extensionListSortable interface{ sort() }

func (list extensionList) init(config *TerragruntConfigFile) {
	for _, item := range list {
		item.init(config, item)
		item.normalize()
	}
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

func merge(list []terragruntExtensioner, imported []terragruntExtensioner, mode mergeMode, argName string) []terragruntExtensioner {
	if len(imported) == 0 {
		return list
	}

	log := imported[0].logger()

	// Create a map with existing elements
	index := make(map[string]int, len(list))
	for i, item := range list {
		index[item.id()] = i
	}

	// Check if there are duplicated elements in the imported list
	indexImported := make(map[string]int, len(list))
	for i, item := range imported {
		indexImported[item.id()] = i
	}

	// Create a list of the elements that should be added to the list
	newList := make([]terragruntExtensioner, 0, len(imported))
	for i, item := range imported {
		name := item.id()
		if pos := indexImported[name]; pos != i {
			log.Warningf("Skipping previous definition of %s %v as it is overridden in the same file", argName, name)
			continue
		}
		if pos, exist := index[name]; exist {
			// It already exist in the list, so is is an override
			// We remove it from its current position and add it to the list of newly added elements to keep its original declaration ordering.
			newList = append(newList, list[pos])
			delete(index, name)
			log.Debugf("Skipping %s %v as it is overridden in the current config", argName, name)
			continue
		}
		newList = append(newList, item)
	}

	if len(list) == 0 {
		return newList
	}

	if len(index) != len(list) {
		// Some elements must be removed from the original list, we simply regenerate the list
		// including only elements that are still in the index.
		newList := make([]terragruntExtensioner, 0, len(index))
		for _, item := range list {
			name := item.id()
			if _, found := index[name]; found {
				newList = append(newList, item)
			}
		}
		list = newList
	}

	if mode == mergeModeAppend {
		return append(list, newList...)
	}
	return append(newList, list...)
}

func help(list extensionListCompatible, listOnly bool, lookups ...string) (result string) {
	if sortable, ok := list.(extensionListSortable); ok {
		sortable.sort()
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
		result += fmt.Sprintf("\n%s%s%s\n%s", TitleID(item.id()), name, extra, specificHelp)
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
			addLine(TitleID(item.id()), name, item.extraInfo())
		}
	}

	for _, item := range list.toGeneric() {
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
