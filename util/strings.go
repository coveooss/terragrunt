package util

import (
	"fmt"
	"reflect"
	"strings"
)

// UnIndent returns the unindented version of the supplied string
func UnIndent(s string) string {
	if !strings.HasPrefix(s, " ") || !strings.Contains(s, "\n") {
		// There is no indentation on the first line
		return s
	}
	split := strings.Split(s, "\n")

	// Count the number of spaces on the first line
	i := 0
	for ; i < len(split[0]); i++ {
		if split[0][i] != ' ' {
			break
		}
	}

	// Remove the indentation (preserving the indentation relative to the first line)
	prefix := strings.Repeat(" ", i)
	for i := range split {
		split[i] = strings.TrimPrefix(split[i], prefix)
	}
	return strings.Join(split, "\n")
}

// Indent returns the indented version of the supplied string
func Indent(s string, indent int) string {
	split := strings.Split(s, "\n")
	space := strings.Repeat(" ", indent)

	for i := range split {
		split[i] = space + split[i]
	}
	return strings.Join(split, "\n")
}

// PrettyPrintStruct returns a readable version of an object
func PrettyPrintStruct(object interface{}) string {
	var out string
	isZero := func(x interface{}) bool {
		return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
	}

	val := reflect.ValueOf(object)
	switch val.Kind() {
	case reflect.Interface:
		fallthrough
	case reflect.Ptr:
		val = val.Elem()
	}

	result := make([][2]string, 0, val.NumField())
	max := 0
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := val.Type().Field(i)

		if !field.CanInterface() {
			continue
		}

		itf := val.Field(i).Interface()
		if isZero(itf) {
			continue
		}

		itf = reflect.Indirect(val.Field(i)).Interface()
		value := strings.Split(strings.TrimSpace(UnIndent(fmt.Sprint(itf))), "\n")
		if val.Field(i).Kind() == reflect.Struct {
			value[0] = "\n" + Indent(strings.Join(value, "\n"), 4)
		} else if len(value) > 1 {
			value[0] += " ..."
		}
		result = append(result, [2]string{fieldType.Name, value[0]})
		if len(fieldType.Name) > max {
			max = len(fieldType.Name)
		}
	}

	for _, entry := range result {
		out += fmt.Sprintf("%*s = %v\n", -max, entry[0], entry[1])
	}

	return out
}
