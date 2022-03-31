package lib

import "strings"

// GlobbedStringsMatch compares item to val with support for a leading and/or
// trailing wildcard '*' in item.
func GlobbedStringsMatch(item, val string) bool {
	if len(item) < 2 {
		return val == item
	}

	hasPrefix := strings.HasPrefix(item, "*")
	hasSuffix := strings.HasSuffix(item, "*")

	if hasPrefix && hasSuffix {
		return strings.Contains(val, item[1:len(item)-1])
	} else if hasPrefix {
		return strings.HasSuffix(val, item[1:])
	} else if hasSuffix {
		return strings.HasPrefix(val, item[:len(item)-1])
	}

	return val == item
}
