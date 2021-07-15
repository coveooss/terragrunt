package config

import (
	"fmt"
	"strings"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/coveooss/multilogger/errors"
	"github.com/coveooss/terragrunt/v2/options"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/zclconf/go-cty/cty"
)

// RunCondition defines the rules that are evaluated in order to determine
type RunCondition struct {
	TerragruntExtension `hcl:",squash"`

	RunIf    cty.Value `hcl:"run_if,optional"`
	IgnoreIf cty.Value `hcl:"ignore_if,optional"`

	allow, deny condition
}

func (c *RunCondition) normalize() error {
	var errors errors.Array
	if err := c.allow.init(c.RunIf); err != nil {
		errors = append(errors, err)
	}
	if err := c.deny.init(c.IgnoreIf); err != nil {
		errors = append(errors, err)
	}
	return errors.AsError()
}

type condition map[string]interface{}

func (c condition) String() string {
	conditions := make([]string, 0, len(c))
	for key, val := range c {
		if list, _ := val.(collections.IGenericList); list != nil {
			if list.Len() == 1 {
				conditions = append(conditions, fmt.Sprintf("%s = %s", key, list.First()))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s in [%s]", key, list.Join(", ")))
			}
		}
	}
	return strings.Join(conditions, " and ")
}

func (c *condition) init(value cty.Value) error {
	if value.IsNull() {
		return nil
	}
	if err := util.FromCtyValue(value, c); err != nil {
		return err
	}

	for key, val := range *c {
		if list, _ := val.([]interface{}); list != nil {
			(*c)[key] = collections.NewList(list...)
		} else {
			(*c)[key] = collections.NewList(fmt.Sprint(val))
		}
	}
	return nil
}

func (c condition) match(options *options.TerragruntOptions) (bool, error) {
	for key, values := range c {
		keyValue, ok := options.GetVariableValue(key)
		if list := values.(collections.IGenericList); !list.Contains(key) { // Direct match
			if !ok {
				return false, fmt.Errorf("variable undefined (%s)", key)
			}
			if !list.Contains(keyValue) {
				return false, nil
			}
		}
	}
	return true, nil
}

var hasDeny = func(c *RunCondition) bool { return len(c.deny) > 0 }
var hasAllow = func(c *RunCondition) bool { return len(c.allow) > 0 }

// ----------------------- RunConditionList -----------------------

//go:generate genny -tag=genny -in=template_extensions.go -out=generated.run_condition.go gen TypeName=RunCondition
func (list RunConditionList) argName() string      { return "run_conditions" }
func (list RunConditionList) mergeMode() mergeMode { return mergeModeAppend }

func (list RunConditionList) String() (result string) {
	var denies, allows []string
	for _, c := range list.Filter(hasDeny) {
		denies = append(denies, fmt.Sprint(c.deny))
	}
	if len(denies) > 0 {
		result += "Deny if " + strings.Join(denies, " or ")
	}

	for _, c := range list.Filter(hasAllow) {
		allows = append(allows, fmt.Sprint(c.allow))
	}
	if len(allows) > 0 {
		if result != "" {
			result += ", "
		}
		result += "Run if " + strings.Join(allows, " or ")
	}
	return
}

// ShouldRun returns whether or not the current project should be run based on its run conditions and the variables in its options.
func (list RunConditionList) ShouldRun() bool {
	for _, c := range list.Filter(hasDeny) {
		if denied, err := c.deny.match(c.options()); err != nil {
			c.logger().Warningf("Ignoring project because %v in %s", err, c.deny)
			return false
		} else if denied {
			c.logger().Warningf("Ignoring project because of ignore rule %s", c.deny)
			return false
		}
	}

	if allows := list.Filter(hasAllow); len(allows) > 0 {
		for _, c := range allows {
			if allowed, err := c.allow.match(c.options()); err != nil {
				c.logger().Warningf("Ignoring project because %v in %s", err, c.allow)
				return false
			} else if allowed {
				return true
			}
		}
		return false // There are allow conditions but no one matched
	}
	return true // There is either no condition or no allow rules, so we return true
}
