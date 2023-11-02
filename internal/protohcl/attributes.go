// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package protohcl

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (u UnmarshalOptions) decodeAttribute(ctx *UnmarshalContext, newMessage newMessageFn, f protoreflect.FieldDescriptor, val cty.Value, listAllowed bool) (protoreflect.Value, error) {
	if f.IsMap() {
		return u.decodeAttributeToMap(ctx, newMessage, f, val)
	}

	if f.IsList() && listAllowed {
		return u.decodeAttributeToList(ctx, newMessage, f, val)
	}

	ok, value, err := decodeAttributeToWellKnownType(f, val)
	if ok {
		return value, errors.Wrapf(err, "%s: Failed to unmarshal argument %s", ctx.ErrorRange(), ctx.Name)
	}

	ok, value, err = u.decodeAttributeToMessage(ctx, newMessage, f, val)
	if ok {
		return value, err
	}

	value, err = decodeAttributeToPrimitive(f, val)
	if err != nil {
		return value, errors.Wrapf(err, "%s: Failed to unmarshal argument %s", ctx.ErrorRange(), ctx.Name)
	}
	return value, nil
}

func (u UnmarshalOptions) decodeAttributeToMessage(ctx *UnmarshalContext, newMessage newMessageFn, desc protoreflect.FieldDescriptor, val cty.Value) (bool, protoreflect.Value, error) {
	if desc.Kind() != protoreflect.MessageKind {
		return false, protoreflect.Value{}, nil
	}

	msg := newMessage().Message()

	ctx = &UnmarshalContext{
		Parent:  ctx.Parent,
		Name:    ctx.Name,
		Message: msg,
		Range:   ctx.Range,
	}

	// We have limited support for HCL functions, essentially just those that
	// return a protobuf message (like the resource `gvk` function) in which
	// case, the message will be wrapped in a cty capsule.
	if val.Type().IsCapsuleType() {
		msg, ok := val.EncapsulatedValue().(proto.Message)
		if ok {
			return true, protoreflect.ValueOf(msg.ProtoReflect()), nil
		} else {
			return true, protoreflect.Value{}, fmt.Errorf("expected encapsulated value to be a message, actual type: %T", val.EncapsulatedValue())
		}
	}

	if !val.Type().IsObjectType() {
		return false, protoreflect.Value{}, nil
	}

	decoder := newObjectDecoder(val, u.FieldNamer, ctx.ErrorRange())

	if err := u.decodeMessage(ctx, decoder, msg); err != nil {
		return true, protoreflect.Value{}, err
	}
	return true, protoreflect.ValueOf(msg), nil
}

func (u UnmarshalOptions) decodeAttributeToList(ctx *UnmarshalContext, newMessage newMessageFn, desc protoreflect.FieldDescriptor, value cty.Value) (protoreflect.Value, error) {
	if value.IsNull() {
		return protoreflect.Value{}, nil
	}

	valueType := value.Type()
	if !valueType.IsListType() && !valueType.IsTupleType() {
		return protoreflect.Value{}, fmt.Errorf("expected list/tuple type after HCL decode but the actual type was %s", valueType.FriendlyName())
	}

	if value.LengthInt() < 1 {
		return protoreflect.Value{}, nil
	}

	protoList := newMessage().List()

	var err error
	var idx int
	value.ForEachElement(func(_ cty.Value, val cty.Value) bool {
		var protoVal protoreflect.Value
		protoVal, err = u.decodeAttribute(&UnmarshalContext{
			Parent: ctx,
			Name:   fmt.Sprintf("%s[%d]", u.FieldNamer.NameField(desc), idx),
		}, protoList.NewElement, desc, val, false)
		if err != nil {
			return true
		}

		idx++
		protoList.Append(protoVal)
		return false
	})
	if err != nil {
		return protoreflect.Value{}, err
	}

	return protoreflect.ValueOfList(protoList), nil
}

func (u UnmarshalOptions) decodeAttributeToMap(ctx *UnmarshalContext, newMessage newMessageFn, desc protoreflect.FieldDescriptor, value cty.Value) (protoreflect.Value, error) {
	if value.IsNull() {
		return protoreflect.Value{}, nil
	}

	valueType := value.Type()
	if !valueType.IsMapType() && !valueType.IsObjectType() {
		return protoreflect.Value{}, fmt.Errorf("expected map/object type after HCL decode but the actual type was %s", valueType.FriendlyName())
	}

	if value.LengthInt() < 1 {
		return protoreflect.Value{}, nil
	}

	protoMap := newMessage().Map()
	protoValueDesc := desc.MapValue()
	var err error

	value.ForEachElement(func(key cty.Value, val cty.Value) (stop bool) {
		var protoVal protoreflect.Value
		protoVal, err = u.decodeAttribute(&UnmarshalContext{
			Parent:  ctx,
			Name:    fmt.Sprintf("%s[%q]", u.FieldNamer.NameField(desc), key.AsString()),
			Message: nil, // TODO: what should this really be?
		}, protoMap.NewValue, protoValueDesc, val, false)
		if err != nil {
			return true
		}
		if protoVal.IsValid() {
			// HCL doesn't support non-string keyed maps so we blindly use string keys
			protoMap.Set(protoreflect.ValueOfString(key.AsString()).MapKey(), protoVal)
		}
		return false
	})
	if err != nil {
		return protoreflect.Value{}, err
	}

	return protoreflect.ValueOfMap(protoMap), nil
}
