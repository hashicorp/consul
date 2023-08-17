package protohcl

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func decodeAttributeToPrimitive(desc protoreflect.FieldDescriptor, val cty.Value) (protoreflect.Value, error) {
	switch kind := desc.Kind(); kind {
	case protoreflect.BoolKind:
		return protoBoolFromCty(val)
	case protoreflect.EnumKind:
		return protoEnumFromCty(desc, val)
	case protoreflect.Int32Kind:
		return protoInt32FromCty(val)
	case protoreflect.Sint32Kind:
		return protoInt32FromCty(val)
	case protoreflect.Uint32Kind:
		return protoUint32FromCty(val)
	case protoreflect.Int64Kind:
		return protoInt64FromCty(val)
	case protoreflect.Sint64Kind:
		return protoInt64FromCty(val)
	case protoreflect.Uint64Kind:
		return protoUint64FromCty(val)
	case protoreflect.Sfixed32Kind:
		return protoInt32FromCty(val)
	case protoreflect.Fixed32Kind:
		return protoUint32FromCty(val)
	case protoreflect.FloatKind:
		return protoFloatFromCty(val)
	case protoreflect.Sfixed64Kind:
		return protoInt64FromCty(val)
	case protoreflect.Fixed64Kind:
		return protoUint64FromCty(val)
	case protoreflect.DoubleKind:
		return protoDoubleFromCty(val)
	case protoreflect.StringKind:
		return protoStringFromCty(val)
	case protoreflect.BytesKind:
		return protoBytesFromCty(val)
	default:
		return protoreflect.Value{}, fmt.Errorf("unknown primitive protobuf kind: %q", kind.String())
	}
}

func protoBoolFromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := boolFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfBool(goVal), nil
}

func protoEnumFromCty(desc protoreflect.FieldDescriptor, val cty.Value) (protoreflect.Value, error) {
	if val.Type() != cty.String {
		return protoreflect.Value{}, fmt.Errorf("expected value of type %s but actual type is %s", cty.String.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		defaultValDesc := desc.DefaultEnumValue()
		return protoreflect.ValueOfEnum(defaultValDesc.Number()), nil
	}

	valDesc := desc.Enum().Values().ByName(protoreflect.Name(val.AsString()))
	if valDesc == nil {
		defaultValDesc := desc.DefaultEnumValue()
		return protoreflect.ValueOfEnum(defaultValDesc.Number()), nil
	}

	return protoreflect.ValueOfEnum(valDesc.Number()), nil
}

func protoInt32FromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := int32FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfInt32(goVal), nil
}

func protoUint32FromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := uint32FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfUint32(goVal), nil
}

func protoInt64FromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := int64FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfInt64(goVal), nil
}

func protoUint64FromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := uint64FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfUint64(goVal), nil
}

func protoFloatFromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := floatFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfFloat32(goVal), nil
}

func protoDoubleFromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := doubleFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfFloat64(goVal), nil
}

func protoStringFromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := stringFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfString(goVal), nil
}

func protoBytesFromCty(val cty.Value) (protoreflect.Value, error) {
	goVal, err := bytesFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoreflect.ValueOfBytes(goVal), nil
}
