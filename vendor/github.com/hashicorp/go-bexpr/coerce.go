package bexpr

import (
	"reflect"
	"strconv"
)

// CoerceInt conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int`
func CoerceInt(value string) (interface{}, error) {
	i, err := strconv.ParseInt(value, 0, 0)
	return int(i), err
}

// CoerceInt8 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int8`
func CoerceInt8(value string) (interface{}, error) {
	i, err := strconv.ParseInt(value, 0, 8)
	return int8(i), err
}

// CoerceInt16 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int16`
func CoerceInt16(value string) (interface{}, error) {
	i, err := strconv.ParseInt(value, 0, 16)
	return int16(i), err
}

// CoerceInt32 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int32`
func CoerceInt32(value string) (interface{}, error) {
	i, err := strconv.ParseInt(value, 0, 32)
	return int32(i), err
}

// CoerceInt64 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int64`
func CoerceInt64(value string) (interface{}, error) {
	i, err := strconv.ParseInt(value, 0, 64)
	return int64(i), err
}

// CoerceUint conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int`
func CoerceUint(value string) (interface{}, error) {
	i, err := strconv.ParseUint(value, 0, 0)
	return uint(i), err
}

// CoerceUint8 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int8`
func CoerceUint8(value string) (interface{}, error) {
	i, err := strconv.ParseUint(value, 0, 8)
	return uint8(i), err
}

// CoerceUint16 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int16`
func CoerceUint16(value string) (interface{}, error) {
	i, err := strconv.ParseUint(value, 0, 16)
	return uint16(i), err
}

// CoerceUint32 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int32`
func CoerceUint32(value string) (interface{}, error) {
	i, err := strconv.ParseUint(value, 0, 32)
	return uint32(i), err
}

// CoerceUint64 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `int64`
func CoerceUint64(value string) (interface{}, error) {
	i, err := strconv.ParseUint(value, 0, 64)
	return uint64(i), err
}

// CoerceBool conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into a `bool`
func CoerceBool(value string) (interface{}, error) {
	return strconv.ParseBool(value)
}

// CoerceFloat32 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `float32`
func CoerceFloat32(value string) (interface{}, error) {
	// ParseFloat always returns a float64 but ensures
	// it can be converted to a float32 without changing
	// its value
	f, err := strconv.ParseFloat(value, 32)
	return float32(f), err
}

// CoerceFloat64 conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into an `float64`
func CoerceFloat64(value string) (interface{}, error) {
	return strconv.ParseFloat(value, 64)
}

// CoerceString conforms to the FieldValueCoercionFn signature
// and can be used to convert the raw string value of
// an expression into a `string`
func CoerceString(value string) (interface{}, error) {
	return value, nil
}

var primitiveCoercionFns = map[reflect.Kind]FieldValueCoercionFn{
	reflect.Bool:    CoerceBool,
	reflect.Int:     CoerceInt,
	reflect.Int8:    CoerceInt8,
	reflect.Int16:   CoerceInt16,
	reflect.Int32:   CoerceInt32,
	reflect.Int64:   CoerceInt64,
	reflect.Uint:    CoerceUint,
	reflect.Uint8:   CoerceUint8,
	reflect.Uint16:  CoerceUint16,
	reflect.Uint32:  CoerceUint32,
	reflect.Uint64:  CoerceUint64,
	reflect.Float32: CoerceFloat32,
	reflect.Float64: CoerceFloat64,
	reflect.String:  CoerceString,
}
