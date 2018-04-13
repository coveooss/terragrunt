package util

import (
	goErrors "errors"
	"fmt"
	"strings"
)

// ListContainsElement returns true if the given list contains the given element
func ListContainsElement(list []string, element string) bool {
	for _, item := range list {
		if item == element {
			return true
		}
	}

	return false
}

// ListContainsElementInterface returns true if the given list contains the given element
func ListContainsElementInterface(list []interface{}, element interface{}) bool {
	for _, item := range list {
		if item == element {
			return true
		}
	}

	return false
}

// Return a copy of the given list with all instances of the given element removed
func RemoveElementFromList(list []string, element string) []string {
	return removeElementFromList(list, element, wholeValue)
}

// Returns a copy of the given list with all duplicates removed (keeping the first encountereds)
func RemoveDuplicatesFromListKeepFirst(list []string) []string {
	return RemoveDuplicatesFromList(list, false, wholeValue)
}

// Returns a copy of the given list with all duplicates removed (keeping the last encountereds)
func RemoveDuplicatesFromListKeepLast(list []string) []string {
	return RemoveDuplicatesFromList(list, true, wholeValue)
}

// Returns a copy of the given list with all duplicates removed (keeping the last encountereds)
// Params:
//   list: The list to filter
//   keepLast: Indicates whether the last or first encountered duplicate element should be kept
//	 getKey: Function used to extract the actual key from the string
func RemoveDuplicatesFromList(list []string, keepLast bool, getKey func(key string) string) []string {
	out := make([]string, 0, len(list))
	present := make(map[string]bool)

	for _, value := range list {
		if _, ok := present[getKey(value)]; ok {
			if keepLast {
				out = removeElementFromList(out, value, getKey)
			} else {
				continue
			}
		}
		out = append(out, value)
		present[value] = true
	}
	return out
}

func removeElementFromList(list []string, element string, getKey func(key string) string) []string {
	out := []string{}
	for _, item := range list {
		if item != element {
			out = append(out, item)
		}
	}
	return out
}

func wholeValue(key string) string { return key }

// CommaSeparatedStrings returns an HCL compliant formated list of strings (each string within double quote)
func CommaSeparatedStrings(list []string) string {
	values := make([]string, 0, len(list))
	for _, value := range list {
		values = append(values, fmt.Sprintf(`"%s"`, value))
	}
	return strings.Join(values, ", ")
}

// SplitEnvVariable returns the two parts of an environment variable
func SplitEnvVariable(str string) (key, value string, err error) {
	variableSplit := strings.SplitN(str, "=", 2)

	if len(variableSplit) == 2 {
		key, value, err = strings.TrimSpace(variableSplit[0]), variableSplit[1], nil
	} else {
		err = goErrors.New(fmt.Sprintf("Invalid variable format %v, should be name=value", str))
	}
	return
}

// IndexOrDefault returns the item at index position or default value if the array is shorter than index
func IndexOrDefault(list []string, index int, defVal string) string {
	if len(list) > index {
		return list[index]
	}
	return defVal
}
