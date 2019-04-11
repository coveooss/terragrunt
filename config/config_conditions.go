package config

import (
	"fmt"
	"strings"

	"github.com/coveo/gotemplate/v3/collections"
	"github.com/coveo/gotemplate/v3/errors"
	"github.com/gruntwork-io/terragrunt/options"
)

// RunConditions defines the rules that are evaluated in order to determine
type RunConditions struct {
	Allows []Condition `hcl:"run_if"`
	Denies []Condition `hcl:"ignore_if"`
}

func (c RunConditions) String() (result string) {
	var sep string
	if len(c.Denies) > 0 {
		sep = "\n"
		result += fmt.Sprintf("Ignore if %s", c.rules(c.Denies))
	}
	if len(c.Allows) > 0 {
		result += fmt.Sprintf("%sRun if %s", sep, c.rules(c.Allows))
	}
	return result
}

// ShouldRun returns whether or not the current project should be run based on its run conditions and the variables in its options.
func (c RunConditions) ShouldRun() bool {
	answer := true
	for _, deny := range c.Denies {
		ok, err := deny.isTrue()
		if err != nil {
			deny.options().Logger.Warningf("Ignoring project because %v in %s", err, deny)
			answer = false
		}
		if ok {
			deny.options().Logger.Warningf("Ignoring project because of ignore rule %s", deny)
			answer = false
		}
	}
	if !answer {
		// There is at least one deny condition
		return false
	}

	if len(c.Allows) == 0 {
		// There is no run_if condition, so we
		return true
	}

	options, hasErr := c.Allows[0].options(), false
	answer = false
	for _, allow := range c.Allows {
		if result, err := allow.isTrue(); err != nil {
			allow.options().Logger.Warningf("Ignoring project because %v in %s", err, allow)
			hasErr = true
		} else if result {
			answer = true
		}
	}

	if answer && !hasErr {
		return true
	}

	if !answer {
		options.Logger.Warningf("Ignoring project because running condition is not met: %s", c.rules(c.Allows))
	}
	return false
}

// Merge combines RunConditions from another source into the current one
func (c *RunConditions) Merge(imported RunConditions) {
	c.Allows = append(c.Allows, imported.Allows...)
	c.Denies = append(c.Denies, imported.Denies...)
}

func (c RunConditions) rules(conditions []Condition) string {
	result := make([]string, len(conditions))
	for i, condition := range conditions {
		result[i] = fmt.Sprint(condition)
	}
	return strings.Join(result, " or ")
}

func (c *RunConditions) init(options *options.TerragruntOptions) (err error) {
	if options.TerragruntRawConfig == nil {
		// If there is no TerragruntRawConfig, we do not have to initialize the RunConditions
		// object, we are probably in test context.
		return nil
	}
	// There is a problem with the native hcl parser and we lost grouping in conditions
	// so, we reinitialize the structure with the raw information
	defer func() { err = errors.Trap(err, recover()) }()
	dict := func(i interface{}) map[string]interface{} { return collections.AsDictionary(i).AsMap() }
	list := func(i interface{}) []interface{} { return collections.AsList(i).AsArray() }

	conditions := list(options.TerragruntRawConfig.Get("run_conditions"))
	c.Allows, c.Denies = nil, nil
	for _, condition := range conditions {
		condition := dict(condition)
		for _, allow := range list(condition["run_if"]) {
			c.Allows = append(c.Allows, Condition(dict(allow)).init(options))
		}
		for _, deny := range list(condition["ignore_if"]) {
			c.Denies = append(c.Denies, Condition(dict(deny)).init(options))
		}
	}
	return
}

// Condition defines a single condition
type Condition map[string]interface{}

func (c Condition) options() *options.TerragruntOptions {
	return c[optionsConfigKey].(*options.TerragruntOptions)
}

func (c Condition) getVariable(v string) string {
	options := c.options()
	if options == nil {
		return v
	}
	if variable, found := options.Variables[v]; found {
		return fmt.Sprint(variable.Value)
	}
	if value := SubstituteVars(v, options); value != v {
		return value
	}
	return noValue
}

const noValue = "<no value>"

func (c Condition) String() string {
	conditions := make([]string, 0, len(c))
	for key, val := range c {
		if list, _ := val.(collections.IGenericList); list != nil {
			keyValue := c.getVariable(key)
			keyValue = collections.IIf(keyValue == noValue, "", fmt.Sprintf("(%s)", keyValue)).(string)
			if list.Len() == 1 {
				conditions = append(conditions, fmt.Sprintf("%s%s = %s", key, keyValue, list.First()))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s%s in [%s]", key, keyValue, list.Join(", ")))
			}
		}
	}
	return strings.Join(conditions, " and ")
}

const optionsConfigKey = "#config!"

func (c Condition) init(options *options.TerragruntOptions) Condition {
	for key, val := range c {
		list, _ := val.(collections.IGenericList)
		if list != nil {
			c[key] = collections.AsList(list)
		} else {
			c[key] = collections.NewList(fmt.Sprint(val))
		}
	}
	c[optionsConfigKey] = options
	return c
}

func (c Condition) isTrue() (bool, error) {
	for key, values := range c {
		if list, _ := values.(collections.IGenericList); list != nil {
			v := c.getVariable(key)
			if strings.Contains(v, noValue) {
				return false, fmt.Errorf("variable undefined (%s)", key)
			}
			if !list.Contains(v) {
				return false, nil
			}
		}
	}
	return true, nil
}
