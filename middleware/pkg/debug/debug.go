package debug

import "strings"

const Name = "o-o.debug."

// IsDebug checks if name is a debugging name, i.e. starts with o-o.debug.
// it returns the empty string if it is not a debug message, otherwise it will return the
// name with o-o.debug. stripped off. Must be called with name lowercased.
func IsDebug(name string) string {
	if len(name) == len(Name) {
		return ""
	}
	name = strings.ToLower(name)
	debug := strings.HasPrefix(name, Name)
	if !debug {
		return ""
	}
	return name[len(Name):]
}
