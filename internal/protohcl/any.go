package protohcl

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
)

const wellKnownTypeAny = "google.protobuf.Any"

type AnyTypeProvider interface {
	AnyType(*UnmarshalContext, MessageDecoder) (protoreflect.FullName, MessageDecoder, error)
}

type AnyTypeURLProvider struct {
	TypeURLFieldName string
}

func (p *AnyTypeURLProvider) AnyType(ctx *UnmarshalContext, decoder MessageDecoder) (protoreflect.FullName, MessageDecoder, error) {
	typeURLFieldName := "type_url"
	if p != nil {
		typeURLFieldName = p.TypeURLFieldName
	}

	var typeURL *IterField
	err := decoder.EachField(FieldIterator{
		Desc: (&anypb.Any{}).ProtoReflect().Descriptor(),
		Func: func(field *IterField) error {
			if field.Name == typeURLFieldName {
				typeURL = field
			}
			return nil
		},
		IgnoreUnknown: true,
	})
	if err != nil {
		return "", nil, err
	}

	if typeURL == nil || typeURL.Val == nil {
		return "", nil, fmt.Errorf("%s field is required to decode Any", typeURLFieldName)
	}

	url, err := stringFromCty(*typeURL.Val)
	if err != nil {
		return "", nil, err
	}

	slashIdx := strings.LastIndex(url, "/")
	typeName := url
	// strip all "hostname" parts of the URL path
	if slashIdx > 1 && slashIdx+1 < len(url) {
		typeName = url[slashIdx+1:]
	}

	return protoreflect.FullName(typeName), decoder.SkipFields(typeURLFieldName), nil
}

func (u UnmarshalOptions) decodeAny(ctx *UnmarshalContext, decoder MessageDecoder, msg protoreflect.Message) error {
	var typeProvider AnyTypeProvider = &AnyTypeURLProvider{TypeURLFieldName: "type_url"}
	if u.AnyTypeProvider != nil {
		typeProvider = u.AnyTypeProvider
	}

	var (
		typeName protoreflect.FullName
		err      error
	)
	typeName, decoder, err = typeProvider.AnyType(ctx, decoder)
	if err != nil {
		return fmt.Errorf("error getting type for Any field: %w", err)
	}

	// the type.googleapis.come/ should be optional
	mt, err := protoregistry.GlobalTypes.FindMessageByName(typeName)
	if err != nil {
		return fmt.Errorf("error looking up type information for %s: %w", typeName, err)
	}

	newMsg := mt.New()

	err = u.decodeMessage(&UnmarshalContext{
		Parent:  ctx.Parent,
		Name:    ctx.Name,
		Message: newMsg,
	}, decoder, newMsg)
	if err != nil {
		return err
	}

	enc, err := proto.Marshal(newMsg.Interface())
	if err != nil {
		return fmt.Errorf("error marshalling Any data as protobuf value: %w", err)
	}

	anyValue := msg.Interface().(*anypb.Any)

	// This will look like <proto package>.<proto Message name> and not quite like a full URL with a path
	anyValue.TypeUrl = string(newMsg.Descriptor().FullName())
	anyValue.Value = enc

	return nil
}

func isAnyField(desc protoreflect.FieldDescriptor) bool {
	if desc.Kind() != protoreflect.MessageKind {
		return false
	}
	return desc.Message().FullName() == wellKnownTypeAny
}
