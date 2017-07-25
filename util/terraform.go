package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl"
)

// FlattenHCL - Convert array of map to single map if there is only one element in the array
// By default, the hcl.Unmarshal returns array of map even if there is only a single map in the definition
func FlattenHCL(source map[string]interface{}) map[string]interface{} {
	for key, value := range source {
		switch value := value.(type) {
		case []map[string]interface{}:
			switch len(value) {
			case 1:
				source[key] = FlattenHCL(value[0])
			default:
				for i, subMap := range value {
					value[i] = FlattenHCL(subMap)
				}
			}
		}
	}
	return source
}

// LoadDefaultValues returns a map of the variables defined in the tfvars file
func LoadDefaultValues(folder string) (result map[string]interface{}, err error) {
	result = map[string]interface{}{}
	for _, file := range getTerraformFiles(folder) {
		var fileVars map[string]interface{}
		switch filepath.Ext(file) {
		case ".tf":
			fileVars, err = getDefaultVars(file, hcl.Unmarshal)
		case ".json":
			fileVars, err = getDefaultVars(file, json.Unmarshal)
		}
		if err != nil {
			return
		}

		for key, value := range fileVars {
			result[key] = value
		}
	}

	return result, nil
}

// LoadTfVars returns a map of the variables defined in the tfvars file
func LoadTfVars(path string) (map[string]interface{}, error) {
	variables := map[string]interface{}{}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return variables, err
	}

	err = hcl.Unmarshal(bytes, &variables)
	return FlattenHCL(variables), err
}

// MarshalHCLVars serialize values to hcl variable definition format
func MarshalHCLVars(value interface{}, indent int) []byte {
	if indent == 0 {
		value = FlattenHCL(value.(map[string]interface{}))
	}
	var buffer bytes.Buffer

	typ, val := reflect.TypeOf(value), reflect.ValueOf(value)
	switch typ.Kind() {
	case reflect.String:
		buffer.WriteByte('"')
		buffer.WriteString(val.String())
		buffer.WriteByte('"')

	case reflect.Int:
		fallthrough
	case reflect.Float64:
		fallthrough
	case reflect.Bool:
		buffer.WriteString(fmt.Sprintf("%v", value))

	case reflect.Slice:
		fallthrough
	case reflect.Array:
		switch val.Len() {
		case 0:
			buffer.WriteString("[]")
		case 1:
			buffer.WriteByte('[')
			buffer.Write(MarshalHCLVars(reflect.ValueOf(value).Index(0).Interface(), indent+1))
			buffer.WriteByte(']')
		default:
			if indent > 0 {
				buffer.WriteString("[\n")
			}
			for i := 0; i < val.Len(); i++ {
				buffer.WriteString(strings.Repeat(" ", indent*2))
				buffer.Write(MarshalHCLVars(val.Index(i).Interface(), indent+1))
				buffer.WriteString(",\n")
			}
			if indent > 0 {
				buffer.WriteString(strings.Repeat(" ", (indent-1)*2))
				buffer.WriteString("]")
			}
		}

	case reflect.Map:
		switch value := value.(type) {
		case map[string]interface{}:
			keys := make([]string, 0, len(value))

			for _, key := range val.MapKeys() {
				keys = append(keys, key.String())
			}
			sort.Strings(keys)

			if indent > 0 {
				buffer.WriteString("{\n")
			}
			for _, key := range keys {
				buffer.WriteString(strings.Repeat(" ", indent*2))
				buffer.WriteString(key)
				buffer.WriteString(" = ")
				buffer.Write(MarshalHCLVars(value[key], indent+1))
				buffer.WriteString("\n")
			}
			if indent > 0 {
				buffer.WriteString(strings.Repeat(" ", (indent-1)*2))
				buffer.WriteString("}")
			}
		}

	default:
		fmt.Printf("Unknown type %T %v : %v\n", value, typ.Kind(), value)
		buffer.WriteString(fmt.Sprintf("%v", value))
	}

	return buffer.Bytes()
}

// Returns the list of terraform files in a folder in alphabetical order (override files are always at the end)
func getTerraformFiles(folder string) []string {
	matches := map[string]int{}

	// Resolve all patterns and add them to the matches map. Since the order is important (i.e. override files comes after non
	// overridden files, we store the last pattern index in the map). f_override.tf will match both *.tf and *_override.tf, but
	// will be associated with *_override.tf at the end, which is what is expected.
	for i, pattern := range patterns {
		files, err := filepath.Glob(filepath.Join(folder, pattern))
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			matches[file] = i
		}
	}

	// Then, we group files in two categories (regular and override) and we sort them alphabetically
	var regularsFiles, overrideFiles []string
	for file, index := range matches {
		list := &regularsFiles
		if index >= 2 {
			// We group overrides files together
			list = &overrideFiles
		}
		*list = append(*list, file)
	}
	sort.Strings(regularsFiles)
	sort.Strings(overrideFiles)
	return append(regularsFiles, overrideFiles...)
}

var patterns = []string{"*.tf", "*.tf.json", "override.tf", "override.tf.json", "*_override.tf", "*_override.tf.json"}

func getDefaultVars(filename string, unmarshal func([]byte, interface{}) error) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	content := map[string]interface{}{}
	if err := unmarshal(bytes, &content); err != nil {
		_, filename = filepath.Split(filename)
		return nil, fmt.Errorf("%v %v", filename, err)
	}

	result := map[string]interface{}{}

	switch variables := content["variable"].(type) {
	case map[string]interface{}:
		for name, value := range variables {
			value := value.(map[string]interface{})

			if value := value["default"]; value != nil {
				result[name] = value
			}
		}
		return result, nil
	case []map[string]interface{}:
		for _, value := range variables {
			for name, value := range value {
				value := value.([]map[string]interface{})[0]

				if value := value["default"]; value != nil {
					result[name] = value
				}
			}
		}
	case nil:
	default:
		return nil, fmt.Errorf("%v: Unknown type %T", filename, variables)
	}
	return result, nil
}
