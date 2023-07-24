package protohcl

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type oneOfTracker struct {
	set map[protoreflect.FullName]string
}

func newOneOfTracker() *oneOfTracker {
	return &oneOfTracker{set: make(map[protoreflect.FullName]string)}
}

func (o *oneOfTracker) markFieldAsSet(desc protoreflect.FieldDescriptor) error {
	oneof := desc.ContainingOneof()
	if oneof == nil {
		return nil
	}

	oneOfName := oneof.FullName()

	if otherFieldName, ok := o.set[oneOfName]; ok {
		oneOfFields := oneof.Fields()
		var builder strings.Builder

		for i := 0; i < oneOfFields.Len(); i++ {
			if i == oneOfFields.Len()-1 {
				builder.WriteString(fmt.Sprintf("%q", oneOfFields.Get(i).TextName()))
			} else if i == oneOfFields.Len()-2 {
				builder.WriteString(fmt.Sprintf("%q and ", oneOfFields.Get(i).TextName()))
			} else {
				builder.WriteString(fmt.Sprintf("%q, ", oneOfFields.Get(i).TextName()))
			}
		}

		return fmt.Errorf("Cannot set %q because %q was previously set. Only one of %s may be set.", desc.TextName(), otherFieldName, builder.String())
	}

	o.set[oneOfName] = desc.TextName()
	return nil
}
