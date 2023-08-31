// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"fmt"
	"reflect"
)

// Merge recursively combines a set of config file structures into a single structure
// according to the following rules:
//
// * only values of type struct, slice, map and pointer to simple types are allowed. Other types panic.
// * when merging two structs the result is the recursive merge of all fields according to the rules below
// * when merging two slices the result is the second slice appended to the first
// * when merging two maps the result is the second map overlaid on the first
// * when merging two pointer values the result is the second value if it is not nil, otherwise the first
func Merge(files ...Config) Config {
	var a Config
	for _, b := range files {
		a = merge(a, b).(Config)
	}
	return a
}

func merge(a, b interface{}) interface{} {
	return mergeValue(reflect.ValueOf(a), reflect.ValueOf(b)).Interface()
}

func mergeValue(a, b reflect.Value) reflect.Value {
	switch a.Kind() {
	case reflect.Map:
		// dont bother allocating a new map to aggregate keys in when either one
		// or both of the maps to merge is the zero value - nil
		if a.IsZero() {
			return b
		} else if b.IsZero() {
			return a
		}

		r := reflect.MakeMap(a.Type())
		for _, k := range a.MapKeys() {
			v := a.MapIndex(k)
			r.SetMapIndex(k, v)
		}
		for _, k := range b.MapKeys() {
			v := b.MapIndex(k)
			r.SetMapIndex(k, v)
		}
		return r

	case reflect.Ptr:
		if !b.IsNil() {
			return b
		}
		return a

	case reflect.Slice:
		if !a.IsValid() {
			a = reflect.Zero(a.Type())
		}
		return reflect.AppendSlice(a, b)

	case reflect.Struct:
		r := reflect.New(a.Type()) // &struct{}
		for i := 0; i < a.NumField(); i++ {
			v := mergeValue(a.Field(i), b.Field(i))
			r.Elem().Field(i).Set(v)
		}
		return r.Elem() // *struct

	default:
		panic(fmt.Sprintf("unsupported element type: %v", a.Type()))
	}
}
