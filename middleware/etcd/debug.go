package etcd

import "strings"

const debugName = "o-o.debug."

// isDebug checks if name is a debugging name, i.e. starts with o-o.debug.
// it return the empty string if it is not a debug message, otherwise it will return the
// name with o-o.debug. stripped off. Must be called with name lowercased.
func isDebug(name string) string {
	if len(name) == len(debugName) {
		return ""
	}
	name = strings.ToLower(name)
	debug := strings.HasPrefix(name, debugName)
	if !debug {
		return ""
	}
	return name[len(debugName):]
}
