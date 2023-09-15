// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package propertyoverride

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// PatchStruct patches the given ProtoMessage according to the documented behavior of Patch and returns the result.
// If an error is returned, the returned K (which may be a partially modified copy) should be ignored.
func PatchStruct[K proto.Message](k K, patch Patch, debug bool) (result K, e error) {
	// Don't panic due to misconfiguration.
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("unexpected panic: %v", err)
		}
	}()

	protoM := k.ProtoReflect()

	if patch.Path == "" || patch.Path == "/" {
		// Return error with all possible fields at root of target.
		return k, fmt.Errorf("non-empty, non-root Path is required;\n%s",
			fieldListStr(protoM.Descriptor(), debug))
	}

	parsedPath := parsePath(patch.Path)
	targetM, fieldDesc, err := findTargetMessageAndField(protoM, parsedPath, patch, debug)
	if err != nil {
		return k, err
	}
	if err := patchField(targetM, fieldDesc, patch, debug); err != nil {
		return k, err
	}

	// ProtoReflect returns a reflective _view_ of the underlying K, so
	// we don't need to convert back explictly when we return k here.
	return k, nil
}

// parsePath returns the path tokens from a JSON Pointer (https://datatracker.ietf.org/doc/html/rfc6901/) string as
// expected by JSON Patch (e.g. "/foo/bar/baz").
func parsePath(path string) []string {
	return strings.Split(strings.TrimLeft(path, "/"), "/")
}

// findTargetMessageAndField takes a root-level protoreflect.Message and slice of path elements and returns the
// protoreflect.FieldDescriptor that corresponds to the full path, along with the parent protoreflect.Message of
// that field. parsedPath must be non-empty.
// If any field in parsedPath cannot be matched while traversing the tree of messages, an error is returned.
func findTargetMessageAndField(m protoreflect.Message, parsedPath []string, patch Patch, debug bool) (protoreflect.Message, protoreflect.FieldDescriptor, error) {
	if len(parsedPath) == 0 {
		return nil, nil, fmt.Errorf("unexpected error: non-empty path is required")
	}

	// Iterate until we've matched the (potentially nested) target field.
	for {
		fieldName := parsedPath[0]
		if fieldName == "" {
			return nil, nil, fmt.Errorf("empty field name in path")
		}

		fieldDesc, err := childFieldDescriptor(m.Descriptor(), fieldName, debug)
		if err != nil {
			return nil, nil, err
		}

		parsedPath = parsedPath[1:]
		if len(parsedPath) == 0 {
			// We've reached the end of the path, return current message and target field.
			return m, fieldDesc, nil
		}

		// Check whether we have a non-terminal (parent) field in the path for which we
		// don't support child operations.
		switch {
		case fieldDesc.IsList():
			return nil, nil, fmt.Errorf("path contains member of repeated field '%s'; repeated field member access is not supported",
				fieldName)
		case fieldDesc.IsMap():
			return nil, nil, fmt.Errorf("path contains member of map field '%s'; map field member access is not supported",
				fieldName)
		case fieldDesc.Message() != nil && fieldDesc.Message().FullName() == "google.protobuf.Any":
			// Return a more helpful error for Any fields early.
			//
			// Doing this here prevents confusing two-step errors, e.g. "no match for field @type"
			// on Any, when in fact we don't support variant proto message fields like Any in general.
			// Because Any is a Message, we'd fail on invalid child fields or unsupported bytes target
			// fields first.
			//
			// In the future, we could support Any by using the type field to initialize a struct for
			// the nested message value.
			return nil, nil, fmt.Errorf("variant-type message fields (google.protobuf.Any) are not supported")
		case !(fieldDesc.Kind() == protoreflect.MessageKind):
			// Non-Any fields that could be used to serialize protos as bytes will get a clear error message
			// in this scenario. This also catches accidental use of non-complex fields as parent fields.
			return nil, nil, fmt.Errorf("path contains member of non-message field '%s' (type '%s'); this type does not support child fields", fieldName, fieldDesc.Kind())
		}

		fieldM := m.Get(fieldDesc).Message()
		if !fieldM.IsValid() && patch.Op == OpAdd {
			// Init this message field to a valid, empty value so that we can keep walking
			// the path and set inner fields. Only do this for add, as remove should not
			// initialize fields on its own.
			m.Set(fieldDesc, protoreflect.ValueOfMessage(fieldM.New()))
			fieldM = m.Get(fieldDesc).Message()
		}

		// Advance our parent message "pointer" to the next field.
		m = fieldM
	}
}

// patchField applies the given patch op to the target field on given parent message.
func patchField(parentM protoreflect.Message, fieldDesc protoreflect.FieldDescriptor, patch Patch, debug bool) error {
	switch patch.Op {
	case OpAdd:
		return applyAdd(parentM, fieldDesc, patch, debug)
	case OpRemove:
		// Ignore Value if provided, per JSON Patch: "Members that are not explicitly defined for the
		// operation in question MUST be ignored (i.e., the operation will complete as if the undefined
		// member did not appear in the object)."
		return applyRemove(parentM, fieldDesc)
	}
	return fmt.Errorf("unexpected error: no op implementation found")
}

// removeField clears the target field on the given message.
func applyRemove(parentM protoreflect.Message, fieldDesc protoreflect.FieldDescriptor) error {
	// Check whether the parent has this field, as clearing a field on an unset parent may panic.
	if parentM.Has(fieldDesc) {
		parentM.Clear(fieldDesc)
	}
	return nil
}

// applyAdd updates the target field(s) on the given message based on the content of patch.
// If the patch value is a scalar, scalar wrapper, or scalar array, we set the target field directly with that value.
// If the patch value is a map, we set the indicated child fields on a new (empty) message matching the target field.
// Regardless, the target field is replaced entirely. This conforms to the PUT-style semantics of the JSON Patch "add"
// operation for objects (https://www.rfc-editor.org/rfc/rfc6902#section-4.1).
func applyAdd(parentM protoreflect.Message, fieldDesc protoreflect.FieldDescriptor, patch Patch, debug bool) error {
	if patch.Value == nil {
		return fmt.Errorf("non-nil Value is required; use an empty map to reset all fields on a message or the 'remove' op to unset fields")
	}
	mapValue, isMapValue := patch.Value.(map[string]interface{})
	// If the field is a proto map type, we'll treat it as a "single" field for error handling purposes.
	// If we support proto map targets in the future, it will still likely be treated as a single field,
	// similar to a list (repeated field). This map handling is specific to _our_ patch semantics for
	// updating multiple message fields at once.
	if isMapValue && !fieldDesc.IsMap() {
		if fieldDesc.Kind() != protoreflect.MessageKind {
			return fmt.Errorf("non-message field type '%s' cannot be set via a map", fieldDesc.Kind())
		}

		// Get a fresh copy of the target field's message, then set the children indicated by the patch.
		fieldM := parentM.Get(fieldDesc).Message().New()
		for k, v := range mapValue {
			targetFieldDesc, err := childFieldDescriptor(fieldDesc.Message(), k, debug)
			if err != nil {
				return err
			}
			val, err := toProtoValue(fieldM, targetFieldDesc, v)
			if err != nil {
				return err
			}
			fieldM.Set(targetFieldDesc, val)
		}
		parentM.Set(fieldDesc, protoreflect.ValueOf(fieldM))

	} else {
		// Just set the field directly, as our patch value is not a map.
		val, err := toProtoValue(parentM, fieldDesc, patch.Value)
		if err != nil {
			return err
		}
		parentM.Set(fieldDesc, val)
	}

	return nil
}

func childFieldDescriptor(parentDesc protoreflect.MessageDescriptor, fieldName string, debug bool) (protoreflect.FieldDescriptor, error) {
	if childFieldDesc := parentDesc.Fields().ByName(protoreflect.Name(fieldName)); childFieldDesc != nil {
		return childFieldDesc, nil
	}
	return nil, fmt.Errorf("no match for field '%s'!\n%s", fieldName, fieldListStr(parentDesc, debug))
}

// fieldListStr prints all possible fields (debug) or the first 10 fields of a given MessageDescriptor.
func fieldListStr(messageDesc protoreflect.MessageDescriptor, debug bool) string {
	// Future: it might be nice to use something like https://github.com/agnivade/levenshtein
	// or https://github.com/schollz/closestmatch to inform these choices.
	fields := messageDesc.Fields()

	// If we're not in debug mode and there's > 10 possible fields, only print the first 10.
	printCount := fields.Len()
	truncateFields := false
	if !debug && printCount > 10 {
		truncateFields = true
		printCount = 10
	}

	msg := strings.Builder{}
	msg.WriteString("available ")
	msg.WriteString(string(messageDesc.FullName()))
	msg.WriteString(" fields:\n")
	for i := 0; i < printCount; i++ {
		msg.WriteString(fields.Get(i).TextName())
		msg.WriteString("\n")
	}
	if truncateFields {
		msg.WriteString("First 10 fields for this message included, configure with `Debug = true` to print all.")
	}

	return msg.String()
}

func toProtoValue(parentM protoreflect.Message, fieldDesc protoreflect.FieldDescriptor, patchValue interface{}) (v protoreflect.Value, e error) {
	// Repeated fields. Check for these first, so we can use Kind below for single value
	// fields (repeated fields have a Kind corresponding to their members).
	// We have to do special handling for int types and strings since they could be enums.
	if fieldDesc.IsList() {
		list := parentM.NewField(fieldDesc).List()
		switch val := patchValue.(type) {
		case []int:
			return toProtoIntOrEnumList(val, list, fieldDesc)
		case []int32:
			return toProtoIntOrEnumList(val, list, fieldDesc)
		case []int64:
			return toProtoIntOrEnumList(val, list, fieldDesc)
		case []uint:
			return toProtoIntOrEnumList(val, list, fieldDesc)
		case []uint32:
			return toProtoIntOrEnumList(val, list, fieldDesc)
		case []uint64:
			return toProtoIntOrEnumList(val, list, fieldDesc)
		case []float32:
			return toProtoNumericList(val, list, fieldDesc)
		case []float64:
			return toProtoNumericList(val, list, fieldDesc)
		case []bool:
			return toProtoList(val, list)
		case []string:
			if fieldDesc.Kind() == protoreflect.EnumKind {
				return toProtoEnumList(val, list, fieldDesc)
			}
			return toProtoList(val, list)
		default:
			if fieldDesc.Kind() == protoreflect.MessageKind ||
				fieldDesc.Kind() == protoreflect.GroupKind ||
				fieldDesc.Kind() == protoreflect.BytesKind {
				return unsupportedTargetTypeErr(fieldDesc)
			}
			return typeMismatchErr(fieldDesc, val)
		}
	}

	switch fieldDesc.Kind() {
	case protoreflect.MessageKind:
		// google.protobuf wrapper types are used for detecting presence of scalars. If the
		// target field is a message, and the patch value is not a map, we assume the user
		// is targeting a wrapper type or has misconfigured the path.
		return toProtoWrapperValue(fieldDesc, patchValue)
	case protoreflect.EnumKind:
		return toProtoEnumValue(fieldDesc, patchValue)
	case protoreflect.Int32Kind,
		protoreflect.Int64Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sint64Kind,
		protoreflect.Uint32Kind,
		protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind,
		protoreflect.Fixed64Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.FloatKind,
		protoreflect.DoubleKind:
		// We have to be careful specifically obtaining the correct proto field type here,
		// since conversion checking by protoreflect is stringent and will not accept e.g.
		// int->uint32 mismatches, or float->int downcasts.
		switch val := patchValue.(type) {
		case int:
			return toProtoNumericValue(fieldDesc, val)
		case int32:
			return toProtoNumericValue(fieldDesc, val)
		case int64:
			return toProtoNumericValue(fieldDesc, val)
		case uint:
			return toProtoNumericValue(fieldDesc, val)
		case uint32:
			return toProtoNumericValue(fieldDesc, val)
		case uint64:
			return toProtoNumericValue(fieldDesc, val)
		case float32:
			return toProtoNumericValue(fieldDesc, val)
		case float64:
			return toProtoNumericValue(fieldDesc, val)
		}
	case protoreflect.BytesKind,
		protoreflect.GroupKind:
		return unsupportedTargetTypeErr(fieldDesc)
	}

	// Fall back to protoreflect.ValueOf, which may panic if an unexpected type is passed.
	defer func() {
		if err := recover(); err != nil {
			_, e = typeMismatchErr(fieldDesc, patchValue)
		}
	}()
	return protoreflect.ValueOf(patchValue), nil
}

func toProtoList[V float32 | float64 | bool | string](vs []V, l protoreflect.List) (protoreflect.Value, error) {
	for _, v := range vs {
		l.Append(protoreflect.ValueOf(v))
	}
	return protoreflect.ValueOfList(l), nil
}

// toProtoIntOrEnumList takes a slice of some integer type V and returns a protoreflect.Value List of either the
// corresponding proto integer type or enum values, depending on the type of the target field.
func toProtoIntOrEnumList[V int | int32 | int64 | uint | uint32 | uint64](vs []V, l protoreflect.List, fieldDesc protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	if fieldDesc.Kind() == protoreflect.EnumKind {
		return toProtoEnumList(vs, l, fieldDesc)
	}
	return toProtoNumericList(vs, l, fieldDesc)
}

// toProtoNumericList takes a slice of some numeric type V and returns a protoreflect.Value List of the corresponding
// proto integer or float values, depending on the type of the target field.
func toProtoNumericList[V int | int32 | int64 | uint | uint32 | uint64 | float32 | float64](vs []V, l protoreflect.List, fieldDesc protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	for _, v := range vs {
		i, err := toProtoNumericValue(fieldDesc, v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		l.Append(i)
	}
	return protoreflect.ValueOfList(l), nil
}

func toProtoEnumList[V int | int32 | int64 | uint | uint32 | uint64 | string](vs []V, l protoreflect.List, fieldDesc protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	for _, v := range vs {
		e, err := toProtoEnumValue(fieldDesc, v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		l.Append(e)
	}
	return protoreflect.ValueOfList(l), nil
}

// toProtoNumericValue aids converting from a Go numeric type to a specific protoreflect.Value type, casting to match
// the target field type.
//
// It supports converting numeric Go slices to List values, and scalar numeric values, in a protoreflect
// validation-compatible manner by replacing the internal conversion logic of protoreflect.ValueOf with a more lenient
// "blind" cast, with the assumption that inputs to structpatcher may have been deserialized in such a way as to make
// strict type checking infeasible, even if the actual value and target type are compatible. This function does _not_
// attempt to handle overflows, only type conversion.
//
// See https://protobuf.dev/programming-guides/proto3/#scalar for canonical proto3 type mappings, reflected here.
func toProtoNumericValue[V int | int32 | int64 | uint | uint32 | uint64 | float32 | float64](fieldDesc protoreflect.FieldDescriptor, v V) (protoreflect.Value, error) {
	switch fieldDesc.Kind() {
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(v)), nil
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(float64(v)), nil
	case protoreflect.Int32Kind:
		return protoreflect.ValueOfInt32(int32(v)), nil
	case protoreflect.Int64Kind:
		return protoreflect.ValueOfInt64(int64(v)), nil
	case protoreflect.Uint32Kind:
		return protoreflect.ValueOfUint32(uint32(v)), nil
	case protoreflect.Uint64Kind:
		return protoreflect.ValueOfUint64(uint64(v)), nil
	case protoreflect.Sint32Kind:
		return protoreflect.ValueOfInt32(int32(v)), nil
	case protoreflect.Sint64Kind:
		return protoreflect.ValueOfInt64(int64(v)), nil
	case protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(v)), nil
	case protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(uint64(v)), nil
	case protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(v)), nil
	case protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(int64(v)), nil
	default:
		// Fall back to protoreflect.ValueOf, which may panic if an unexpected type is passed.
		return protoreflect.ValueOf(v), nil
	}
}

func toProtoEnumValue(fieldDesc protoreflect.FieldDescriptor, patchValue any) (protoreflect.Value, error) {
	// For enums, we accept the field number or a string representation of the name
	// (both supported by protojson). EnumNumber is a type alias for int32, but we
	// may be dealing with a 64-bit number in Go depending on how it was obtained.
	var enumValue protoreflect.EnumValueDescriptor
	switch val := patchValue.(type) {
	case string:
		enumValue = fieldDesc.Enum().Values().ByName(protoreflect.Name(val))
	case int:
		enumValue = fieldDesc.Enum().Values().ByNumber(protoreflect.EnumNumber(val))
	case int32:
		enumValue = fieldDesc.Enum().Values().ByNumber(protoreflect.EnumNumber(val))
	case int64:
		enumValue = fieldDesc.Enum().Values().ByNumber(protoreflect.EnumNumber(val))
	case uint:
		enumValue = fieldDesc.Enum().Values().ByNumber(protoreflect.EnumNumber(val))
	case uint32:
		enumValue = fieldDesc.Enum().Values().ByNumber(protoreflect.EnumNumber(val))
	case uint64:
		enumValue = fieldDesc.Enum().Values().ByNumber(protoreflect.EnumNumber(val))
	}
	if enumValue != nil {
		return protoreflect.ValueOfEnum(enumValue.Number()), nil
	}
	return typeMismatchErr(fieldDesc, patchValue)
}

// toProtoWrapperValue converts possible runtime Go types (per https://protobuf.dev/programming-guides/proto3/#scalar)
// to the appropriate proto wrapper target type based on the name of the given FieldDescriptor.
//
// This function does not attempt to handle overflows, only type conversion.
func toProtoWrapperValue(fieldDesc protoreflect.FieldDescriptor, patchValue any) (protoreflect.Value, error) {
	fullName := string(fieldDesc.Message().FullName())
	if !strings.HasPrefix(fullName, "google.protobuf.") || !strings.HasSuffix(fullName, "Value") {
		return unsupportedTargetTypeErr(fieldDesc)
	}

	switch val := patchValue.(type) {
	case int:
		return toProtoIntWrapperValue(fieldDesc, val)
	case int32:
		return toProtoIntWrapperValue(fieldDesc, val)
	case int64:
		return toProtoIntWrapperValue(fieldDesc, val)
	case uint:
		return toProtoIntWrapperValue(fieldDesc, val)
	case uint32:
		return toProtoIntWrapperValue(fieldDesc, val)
	case uint64:
		return toProtoIntWrapperValue(fieldDesc, val)
	case float32:
		switch fieldDesc.Message().FullName() {
		case "google.protobuf.FloatValue":
			v := wrapperspb.Float(val)
			return protoreflect.ValueOf(v.ProtoReflect()), nil
		case "google.protobuf.DoubleValue":
			v := wrapperspb.Double(float64(val))
			return protoreflect.ValueOf(v.ProtoReflect()), nil
		default:
			// Fall back to int wrapper, since we may actually be targeting this instead.
			// Failure will result in a typical type mismatch error.
			return toProtoIntWrapperValue(fieldDesc, val)
		}
	case float64:
		switch fieldDesc.Message().FullName() {
		case "google.protobuf.FloatValue":
			v := wrapperspb.Float(float32(val))
			return protoreflect.ValueOf(v.ProtoReflect()), nil
		case "google.protobuf.DoubleValue":
			v := wrapperspb.Double(val)
			return protoreflect.ValueOf(v.ProtoReflect()), nil
		default:
			// Fall back to int wrapper, since we may actually be targeting this instead.
			// Failure will result in a typical type mismatch error.
			return toProtoIntWrapperValue(fieldDesc, val)
		}
	case bool:
		switch fieldDesc.Message().FullName() {
		case "google.protobuf.BoolValue":
			v := wrapperspb.Bool(val)
			return protoreflect.ValueOf(v.ProtoReflect()), nil
		}
	case string:
		switch fieldDesc.Message().FullName() {
		case "google.protobuf.StringValue":
			v := wrapperspb.String(val)
			return protoreflect.ValueOf(v.ProtoReflect()), nil
		}
	}
	return typeMismatchErr(fieldDesc, patchValue)
}

func toProtoIntWrapperValue[V int | int32 | int64 | uint | uint32 | uint64 | float32 | float64](fieldDesc protoreflect.FieldDescriptor, v V) (protoreflect.Value, error) {
	switch fieldDesc.Message().FullName() {
	case "google.protobuf.UInt32Value":
		v := wrapperspb.UInt32(uint32(v))
		return protoreflect.ValueOf(v.ProtoReflect()), nil
	case "google.protobuf.UInt64Value":
		v := wrapperspb.UInt64(uint64(v))
		return protoreflect.ValueOf(v.ProtoReflect()), nil
	case "google.protobuf.Int32Value":
		v := wrapperspb.Int32(int32(v))
		return protoreflect.ValueOf(v.ProtoReflect()), nil
	case "google.protobuf.Int64Value":
		v := wrapperspb.Int64(int64(v))
		return protoreflect.ValueOf(v.ProtoReflect()), nil
	default:
		return typeMismatchErr(fieldDesc, v)
	}
}

func typeMismatchErr(fieldDesc protoreflect.FieldDescriptor, v interface{}) (protoreflect.Value, error) {
	return protoreflect.Value{}, fmt.Errorf("patch value type %T could not be applied to target field type '%s'", v, fieldType(fieldDesc))
}

func unsupportedTargetTypeErr(fieldDesc protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	return protoreflect.Value{}, fmt.Errorf("unsupported target field type '%s'", fieldType(fieldDesc))
}

func fieldType(fieldDesc protoreflect.FieldDescriptor) string {
	v := fieldDesc.Kind().String()
	// Scalars have a useful Kind string, but complex fields should have their full name for clarity.
	if fieldDesc.Kind() == protoreflect.MessageKind {
		v = string(fieldDesc.Message().FullName())
	}
	if fieldDesc.IsList() {
		v = "repeated " + v // Kind reflects the type of repeated elements in repeated fields.
	}
	if fieldDesc.IsMap() {
		v = "map" // maps are difficult to get types from, but we don't support them, so return a simple value for now.
	}
	return v
}
