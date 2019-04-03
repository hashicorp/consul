package bexpr

import (
	"fmt"
)

func validateRecurse(ast Expression, fields FieldConfigurations, maxRawValueLength int) (int, error) {
	switch node := ast.(type) {
	case *UnaryExpression:
		switch node.Operator {
		case UnaryOpNot:
			// this is fine
		default:
			return 0, fmt.Errorf("Invalid unary expression operator: %d", node.Operator)
		}

		if node.Operand == nil {
			return 0, fmt.Errorf("Invalid unary expression operand: nil")
		}
		return validateRecurse(node.Operand, fields, maxRawValueLength)
	case *BinaryExpression:
		switch node.Operator {
		case BinaryOpAnd, BinaryOpOr:
			// this is fine
		default:
			return 0, fmt.Errorf("Invalid binary expression operator: %d", node.Operator)
		}

		if node.Left == nil {
			return 0, fmt.Errorf("Invalid left hand side of binary expression: nil")
		} else if node.Right == nil {
			return 0, fmt.Errorf("Invalid right hand side of binary expression: nil")
		}

		leftMatches, err := validateRecurse(node.Left, fields, maxRawValueLength)
		if err != nil {
			return leftMatches, err
		}

		rightMatches, err := validateRecurse(node.Right, fields, maxRawValueLength)
		return leftMatches + rightMatches, err
	case *MatchExpression:
		if len(node.Selector) < 1 {
			return 1, fmt.Errorf("Invalid selector: %q", node.Selector)
		}

		if node.Value != nil && maxRawValueLength != 0 && len(node.Value.Raw) > maxRawValueLength {
			return 1, fmt.Errorf("Value in expression with length %d for selector %q exceeds maximum length of", len(node.Value.Raw), maxRawValueLength)
		}

		// exit early if we have no fields to check against
		if len(fields) < 1 {
			return 1, nil
		}

		configs := fields
		var lastConfig *FieldConfiguration
		// validate the selector
		for idx, field := range node.Selector {
			if fcfg, ok := configs[FieldName(field)]; ok {
				lastConfig = fcfg
				configs = fcfg.SubFields
			} else if fcfg, ok := configs[FieldNameAny]; ok {
				lastConfig = fcfg
				configs = fcfg.SubFields
			} else {
				return 1, fmt.Errorf("Selector %q is not valid", node.Selector[:idx+1])
			}

			// this just verifies that the FieldConfigurations we are using was created properly
			if lastConfig == nil {
				return 1, fmt.Errorf("FieldConfiguration for selector %q is nil", node.Selector[:idx])
			}
		}

		// check the operator
		found := false
		for _, op := range lastConfig.SupportedOperations {
			if op == node.Operator {
				found = true
				break
			}
		}

		if !found {
			return 1, fmt.Errorf("Invalid match operator %q for selector %q", node.Operator, node.Selector)
		}

		// coerce/validate the value
		if node.Value != nil {
			if lastConfig.CoerceFn != nil {
				coerced, err := lastConfig.CoerceFn(node.Value.Raw)
				if err != nil {
					return 1, fmt.Errorf("Failed to coerce value %q for selector %q: %v", node.Value.Raw, node.Selector, err)
				}

				node.Value.Converted = coerced
			}
		} else {
			switch node.Operator {
			case MatchIsEmpty, MatchIsNotEmpty:
				// these don't require values
			default:
				return 1, fmt.Errorf("Match operator %q requires a non-nil value", node.Operator)
			}
		}
		return 1, nil
	}
	return 0, fmt.Errorf("Cannot validate: Invalid AST")
}

func validate(ast Expression, fields FieldConfigurations, maxMatches, maxRawValueLength int) error {
	matches, err := validateRecurse(ast, fields, maxRawValueLength)
	if err != nil {
		return err
	}

	if maxMatches != 0 && matches > maxMatches {
		return fmt.Errorf("Number of match expressions (%d) exceeds the limit (%d)", matches, maxMatches)
	}

	return nil
}
