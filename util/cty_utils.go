package util

import (
	"encoding/json"

	"github.com/zclconf/go-cty/cty"
	ctyJson "github.com/zclconf/go-cty/cty/json"
)

// ToCtyValue converts a go object to a cty Value
// This is a hacky workaround to convert a cty Value to a Go map[string]interface{}. cty does not support this directly
// (https://github.com/hashicorp/hcl2/issues/108) and doing it with gocty.FromCtyValue is nearly impossible, as cty
// requires you to specify all the output types and will error out when it hits interface{}. So, as an ugly workaround,
// we convert the given value to JSON using cty's JSON library and then convert the JSON back to a
// map[string]interface{} using the Go json library.
func ToCtyValue(value interface{}) (*cty.Value, error) {
	resultJSON, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	resultJSONValue := ctyJson.SimpleJSONValue{}
	if err = resultJSONValue.UnmarshalJSON(resultJSON); err != nil {
		return nil, err
	}
	return &resultJSONValue.Value, nil
}

// FromCtyValue converts a cty Value to a go object
// This is a hacky workaround to convert a cty Value to a Go map[string]interface{}. cty does not support this directly
// (https://github.com/hashicorp/hcl2/issues/108) and doing it with gocty.FromCtyValue is nearly impossible, as cty
// requires you to specify all the output types and will error out when it hits interface{}. So, as an ugly workaround,
// we convert the given value to JSON using cty's JSON library and then convert the JSON back to a
// map[string]interface{} using the Go json library.
func FromCtyValue(ctyValue cty.Value, out interface{}) error {
	marshallableVariable := ctyJson.SimpleJSONValue{Value: ctyValue}
	marshalledVariable, err := marshallableVariable.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(marshalledVariable, out)
}
