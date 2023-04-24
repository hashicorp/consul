package pbservice

import (
	fmt "fmt"
	"reflect"

	types "github.com/golang/protobuf/ptypes/struct"
)

// ProtobufTypesStructToMapStringInterface converts a protobuf/types.Struct into a
// map[string]interface{}.
func ProtobufTypesStructToMapStringInterface(s *types.Struct) map[string]interface{} {
	if s == nil {
		return nil
	}
	m := make(map[string]interface{}, len(s.Fields))
	for k, v := range s.Fields {
		m[k] = interfaceFromPBValue(v)
	}
	return m
}

// interfaceFromPBValue converts a protobuf Value into an interface{}
func interfaceFromPBValue(v *types.Value) interface{} {
	if v == nil {
		return nil
	}
	switch k := v.Kind.(type) {
	case *types.Value_NullValue:
		return nil
	case *types.Value_NumberValue:
		return k.NumberValue
	case *types.Value_StringValue:
		return k.StringValue
	case *types.Value_BoolValue:
		return k.BoolValue
	case *types.Value_StructValue:
		return ProtobufTypesStructToMapStringInterface(k.StructValue)
	case *types.Value_ListValue:
		s := make([]interface{}, len(k.ListValue.Values))
		for i, e := range k.ListValue.Values {
			s[i] = interfaceFromPBValue(e)
		}
		return s
	default:
		panic("unknown kind")
	}
}

// MapStringInterfaceToProtobufTypesStruct converts a map[string]interface{} into a proto.Struct
func MapStringInterfaceToProtobufTypesStruct(m map[string]interface{}) *types.Struct {
	if len(m) == 0 {
		return nil
	}

	fields := make(map[string]*types.Value, len(m))
	for k, v := range m {
		fields[k] = interfaceToPBValue(v)
	}
	return &types.Struct{Fields: fields}
}

// SliceToPBListValue converts a []interface{} into a proto.ListValue. It's used
// internally by MapStringInterfaceToProtobufTypesStruct when it encouters slices.
func SliceToPBListValue(s []interface{}) *types.ListValue {
	if len(s) == 0 {
		return nil
	}

	vals := make([]*types.Value, len(s))
	for i, v := range s {
		vals[i] = interfaceToPBValue(v)
	}
	return &types.ListValue{Values: vals}
}

// interfaceToPBValue converts a interface{} into a proto.Value. It attempts to
// do so by type switch and simple casts where possible but falls back to
// reflection if necessary.
func interfaceToPBValue(v interface{}) *types.Value {
	switch v := v.(type) {
	case nil:
		return nil
	case int:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int8:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int32:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint8:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint32:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case float32:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case float64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: v,
			},
		}
	case string:
		return &types.Value{
			Kind: &types.Value_StringValue{
				StringValue: v,
			},
		}
	case error:
		return &types.Value{
			Kind: &types.Value_StringValue{
				StringValue: v.Error(),
			},
		}
	case map[string]interface{}:
		return &types.Value{
			Kind: &types.Value_StructValue{
				StructValue: MapStringInterfaceToProtobufTypesStruct(v),
			},
		}
	case []interface{}:
		return &types.Value{
			Kind: &types.Value_ListValue{
				ListValue: SliceToPBListValue(v),
			},
		}
	default:
		return interfaceToPBValueReflect(reflect.ValueOf(v))
	}
}

// interfaceToPBValueReflect converts a interface{} into a proto.Value using
// reflection.
func interfaceToPBValueReflect(v reflect.Value) *types.Value {
	switch v.Kind() {
	case reflect.Interface:
		return interfaceToPBValue(v.Interface())
	case reflect.Bool:
		return &types.Value{
			Kind: &types.Value_BoolValue{
				BoolValue: v.Bool(),
			},
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v.Int()),
			},
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v.Uint()),
			},
		}
	case reflect.Float32, reflect.Float64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: v.Float(),
			},
		}
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return interfaceToPBValueReflect(reflect.Indirect(v))
	case reflect.Array, reflect.Slice:
		size := v.Len()
		if size == 0 {
			return nil
		}
		values := make([]*types.Value, size)
		for i := 0; i < size; i++ {
			values[i] = interfaceToPBValue(v.Index(i))
		}
		return &types.Value{
			Kind: &types.Value_ListValue{
				ListValue: &types.ListValue{
					Values: values,
				},
			},
		}
	case reflect.Struct:
		t := v.Type()
		size := v.NumField()
		if size == 0 {
			return nil
		}
		fields := make(map[string]*types.Value, size)
		for i := 0; i < size; i++ {
			name := t.Field(i).Name
			// Only include public fields. There may be a better way with struct tags
			// but this works for now.
			if len(name) > 0 && 'A' <= name[0] && name[0] <= 'Z' {
				fields[name] = interfaceToPBValue(v.Field(i))
			}
		}
		if len(fields) == 0 {
			return nil
		}
		return &types.Value{
			Kind: &types.Value_StructValue{
				StructValue: &types.Struct{
					Fields: fields,
				},
			},
		}
	case reflect.Map:
		keys := v.MapKeys()
		if len(keys) == 0 {
			return nil
		}
		fields := make(map[string]*types.Value, len(keys))
		for _, k := range keys {
			if k.Kind() == reflect.String {
				fields[k.String()] = interfaceToPBValue(v.MapIndex(k))
			}
		}
		if len(fields) == 0 {
			return nil
		}
		return &types.Value{
			Kind: &types.Value_StructValue{
				StructValue: &types.Struct{
					Fields: fields,
				},
			},
		}
	default:
		// Last resort
		return &types.Value{
			Kind: &types.Value_StringValue{
				StringValue: fmt.Sprint(v),
			},
		}
	}
}
