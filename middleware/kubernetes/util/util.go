// Package kubernetes/util provides helper functions for the kubernetes middleware
package util

import (
	"strings"
)

// StringInSlice check whether string a is a member of slice.
func StringInSlice(a string, slice []string) bool {
	for _, b := range slice {
		if b == a {
			return true
		}
	}
	return false
}

// SymbolContainsWildcard checks whether symbol contains a wildcard value
func SymbolContainsWildcard(symbol string) bool {
	return (strings.Contains(symbol, WildcardStar) || (symbol == WildcardAny))
}

const (
	WildcardStar = "*"
	WildcardAny  = "any"
)
