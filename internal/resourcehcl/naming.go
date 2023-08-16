package resourcehcl

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// fieldNamer implements protohcl.FieldNamer to name fields using PascalCase
// with support for acroynms (e.g. ID, TCP).
type fieldNamer struct{ acroynms []string }

func (n fieldNamer) NameField(fd protoreflect.FieldDescriptor) string {
	camel := fd.JSONName()
	upper := strings.ToUpper(camel)

	for _, a := range n.acroynms {
		if upper == a {
			return a
		}
	}

	return strings.ToUpper(camel[:1]) + camel[1:]
}

func (n fieldNamer) GetField(fds protoreflect.FieldDescriptors, name string) protoreflect.FieldDescriptor {
	for _, a := range n.acroynms {
		if name == a {
			return fds.ByJSONName(strings.ToLower(a))
		}
	}

	camel := strings.ToLower(name[:1]) + name[1:]
	return fds.ByJSONName(camel)
}
