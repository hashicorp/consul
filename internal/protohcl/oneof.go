// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package protohcl

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type oneOfTracker struct {
	namer FieldNamer
	set   map[protoreflect.FullName]string
}

func newOneOfTracker(namer FieldNamer) *oneOfTracker {
	return &oneOfTracker{
		namer: namer,
		set:   make(map[protoreflect.FullName]string),
	}
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
			name := o.namer.NameField(oneOfFields.Get(i))

			if i == oneOfFields.Len()-1 {
				fmt.Fprintf(&builder, "%q", name)
			} else if i == oneOfFields.Len()-2 {
				fmt.Fprintf(&builder, "%q and ", name)
			} else {
				fmt.Fprintf(&builder, "%q, ", name)
			}
		}

		return fmt.Errorf("Cannot set %q because %q was previously set. Only one of %s may be set.", o.namer.NameField(desc), otherFieldName, builder.String())
	}

	o.set[oneOfName] = o.namer.NameField(desc)
	return nil
}
