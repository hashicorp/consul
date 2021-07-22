package pbservice

import (
	"strings"
)

// UniqueID returns a unique identifier for this CheckServiceNode, which includes
// the node name, service namespace, and service ID.
//
// The returned ID uses slashes to separate the identifiers, however the node name
// may also contain a slash, so it is not possible to parse this identifier to
// retrieve its constituent parts.
//
// This function is similar to structs.UniqueID, however at this time no guarantees
// are made that it will remain the same.
func (m *CheckServiceNode) UniqueID() string {
	if m == nil {
		return ""
	}
	builder := new(strings.Builder)

	switch {
	case m.Node != nil:
		builder.WriteString(m.Node.Partition + "/")
	case m.Service != nil:
		builder.WriteString(m.Service.EnterpriseMeta.Partition + "/")
	}

	if m.Node != nil {
		builder.WriteString(m.Node.Node + "/")
	}
	if m.Service != nil {
		builder.WriteString(m.Service.EnterpriseMeta.Namespace + "/")
		builder.WriteString(m.Service.ID)
	}
	return builder.String()
}
