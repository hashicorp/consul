package template

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
)

// InterpolateHIL processes the string as if it were HIL and interpolates only
// the provided string->string map as possible variables.
func InterpolateHIL(s string, vars map[string]string, lowercase bool) (string, error) {
	if strings.Index(s, "${") == -1 {
		// Skip going to the trouble of parsing something that has no HIL.
		return s, nil
	}

	tree, err := hil.Parse(s)
	if err != nil {
		return "", err
	}

	vm := make(map[string]ast.Variable)
	for k, v := range vars {
		if lowercase {
			v = strings.ToLower(v)
		}
		vm[k] = ast.Variable{
			Type:  ast.TypeString,
			Value: v,
		}
	}

	config := &hil.EvalConfig{
		GlobalScope: &ast.BasicScope{
			VarMap: vm,
		},
	}

	result, err := hil.Eval(tree, config)
	if err != nil {
		return "", err
	}

	if result.Type != hil.TypeString {
		return "", fmt.Errorf("generated unexpected hil type: %s", result.Type)
	}

	return result.Value.(string), nil
}
