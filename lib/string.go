package lib

import (
	"strings"
)

// StrContains checks if a list contains a string
func StrContains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func ToLowerList(l []string) []string {
	var out []string
	for _, value := range l {
		out = append(out, strings.ToLower(value))
	}
	return out
}
