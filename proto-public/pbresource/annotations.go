package pbresource

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

type TypeInferenceError struct {
	TypeName string
}

func (e TypeInferenceError) Error() string {
	return fmt.Sprintf("protobuf type name is not of the form: hashicorp.consul.<api group>.<group version>.<kind>: %q", e.TypeName)
}

func GetResourceSpec(msg protoreflect.MessageDescriptor) (*ResourceTypeSpec, bool, error) {
	name := string(msg.FullName())
	var spec ResourceTypeSpec
	ext := proto.GetExtension(msg.Options(), E_Spec).(*ResourceTypeSpec)
	if ext == nil {
		return nil, false, nil
	}

	spec.Scope = ext.Scope
	spec.DontMapHttp = ext.DontMapHttp
	if ext.Type != nil {
		spec.Type = ext.Type
	} else {
		// Attempt to parse the resource type out of the name
		parts := strings.Split(name, ".")

		if len(parts) != 5 {
			return nil, true, TypeInferenceError{TypeName: name}
		}

		spec.Type = &Type{
			Group:        parts[2],
			GroupVersion: parts[3],
			Kind:         parts[4],
		}
	}

	return &spec, true, nil
}
