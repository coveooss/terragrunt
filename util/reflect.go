package util

import "reflect"

// KindOf returns the kind of the type or Invalid if value is nil
func KindOf(value interface{}) reflect.Kind {
	valueType := reflect.TypeOf(value)
	if valueType == nil {
		return reflect.Invalid
	}
	return valueType.Kind()
}

// ConvertToMap builds a map with the keys supplied
func ConvertToMap(value interface{}, keys ...string) interface{} {
	if len(keys) > 0 {
		return map[string]interface{}{keys[0]: ConvertToMap(value, keys[1:]...)}
	}
	return value
}
