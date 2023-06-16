// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lib

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/mapstructure"
	"github.com/mitchellh/reflectwalk"
)

// MapWalk will traverse through the supplied input which should be a
// map[string]interface{} (or something compatible that we can coerce
// to a map[string]interface{}) and from it create a new map[string]interface{}
// with all internal values coerced to JSON compatible types. i.e. a []uint8
// can be converted (in most cases) to a string so it will not be base64 encoded
// when output in JSON
func MapWalk(input interface{}) (map[string]interface{}, error) {
	mapCopyRaw, err := copystructure.Copy(input)
	if err != nil {
		return nil, err
	}

	mapCopy, ok := mapCopyRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("internal error: input to MapWalk is not a map[string]interface{}")
	}

	if err := reflectwalk.Walk(mapCopy, &mapWalker{}); err != nil {
		return nil, err
	}

	return mapCopy, nil
}

var typMapIfaceIface = reflect.TypeOf(map[interface{}]interface{}{})
var typByteSlice = reflect.TypeOf([]byte{})

// mapWalker implements interfaces for the reflectwalk package
// (github.com/mitchellh/reflectwalk) that can be used to automatically
// make a JSON compatible map safe for JSON usage. This is currently
// targeted at the map[string]interface{}
//
// Most of the implementation here is just keeping track of where we are
// in the reflectwalk process, so that we can replace values. The key logic
// is in Slice() and SliceElem().
//
// In particular we're looking to replace two cases the msgpack codec causes:
//
//	1.) String values get turned into byte slices. JSON will base64-encode
//	    this and we don't want that, so we convert them back to strings.
//
//	2.) Nested maps turn into map[interface{}]interface{}. JSON cannot
//	    encode this, so we need to turn it back into map[string]interface{}.
type mapWalker struct {
	lastValue    reflect.Value        // lastValue of map, required for replacement
	loc, lastLoc reflectwalk.Location // locations
	cs           []reflect.Value      // container stack
	csKey        []reflect.Value      // container keys (maps) stack
	csData       interface{}          // current container data
	sliceIndex   []int                // slice index stack (one for each slice in cs)
}

func (w *mapWalker) Enter(loc reflectwalk.Location) error {
	w.lastLoc = w.loc
	w.loc = loc
	return nil
}

func (w *mapWalker) Exit(loc reflectwalk.Location) error {
	w.loc = reflectwalk.None
	w.lastLoc = reflectwalk.None

	switch loc {
	case reflectwalk.Map:
		w.cs = w.cs[:len(w.cs)-1]
	case reflectwalk.MapValue:
		w.csKey = w.csKey[:len(w.csKey)-1]
	case reflectwalk.Slice:
		// Split any values that need to be split
		w.cs = w.cs[:len(w.cs)-1]
	case reflectwalk.SliceElem:
		w.csKey = w.csKey[:len(w.csKey)-1]
		w.sliceIndex = w.sliceIndex[:len(w.sliceIndex)-1]
	}

	return nil
}

func (w *mapWalker) Map(m reflect.Value) error {
	w.cs = append(w.cs, m)
	return nil
}

func (w *mapWalker) MapElem(m, k, v reflect.Value) error {
	w.csData = k
	w.csKey = append(w.csKey, k)
	w.lastValue = v

	// We're looking specifically for map[interface{}]interface{}, but the
	// values in a map could be wrapped up in interface{} so we need to unwrap
	// that first. Therefore, we do three checks: 1.) is it valid? so we
	// don't panic, 2.) is it an interface{}? so we can unwrap it and 3.)
	// after unwrapping the interface do we have the map we expect?
	if !v.IsValid() {
		return nil
	}

	if v.Kind() != reflect.Interface {
		return nil
	}

	if inner := v.Elem(); inner.IsValid() && inner.Type() == typMapIfaceIface {
		// map[interface{}]interface{}, attempt to weakly decode into string keys
		var target map[string]interface{}
		if err := mapstructure.WeakDecode(v.Interface(), &target); err != nil {
			return err
		}

		m.SetMapIndex(k, reflect.ValueOf(target))
	}

	return nil
}

func (w *mapWalker) Slice(v reflect.Value) error {
	// If we find a []byte slice, it is an HCL-string converted to []byte.
	// Convert it back to a Go string and replace the value so that JSON
	// doesn't base64-encode it.
	if v.Type() == typByteSlice {
		resultVal := reflect.ValueOf(string(v.Interface().([]byte)))
		switch w.lastLoc {
		case reflectwalk.MapKey:
			m := w.cs[len(w.cs)-1]

			// Delete the old value
			var zero reflect.Value
			m.SetMapIndex(w.csData.(reflect.Value), zero)

			// Set the new key with the existing value
			m.SetMapIndex(resultVal, w.lastValue)

			// Set the key to be the new key
			w.csData = resultVal
		case reflectwalk.MapValue:
			// If we're in a map, then the only way to set a map value is
			// to set it directly.
			m := w.cs[len(w.cs)-1]
			mk := w.csData.(reflect.Value)
			m.SetMapIndex(mk, resultVal)
		case reflectwalk.Slice:
			s := w.cs[len(w.cs)-1]
			s.Index(w.sliceIndex[len(w.sliceIndex)-1]).Set(resultVal)
		default:
			return fmt.Errorf("cannot convert []byte")
		}
	}

	w.cs = append(w.cs, v)
	return nil
}

func (w *mapWalker) SliceElem(i int, elem reflect.Value) error {
	w.csKey = append(w.csKey, reflect.ValueOf(i))
	w.sliceIndex = append(w.sliceIndex, i)

	// We're looking specifically for map[interface{}]interface{}, but the
	// values in a slice are wrapped up in interface{} so we need to unwrap
	// that first. Therefore, we do three checks: 1.) is it valid? so we
	// don't panic, 2.) is it an interface{}? so we can unwrap it and 3.)
	// after unwrapping the interface do we have the map we expect?
	if !elem.IsValid() {
		return nil
	}

	if elem.Kind() != reflect.Interface {
		return nil
	}

	if inner := elem.Elem(); inner.Type() == typMapIfaceIface {
		// map[interface{}]interface{}, attempt to weakly decode into string keys
		var target map[string]interface{}
		if err := mapstructure.WeakDecode(inner.Interface(), &target); err != nil {
			return err
		}

		elem.Set(reflect.ValueOf(target))
	} else if inner := elem.Elem(); inner.Type() == typByteSlice {
		elem.Set(reflect.ValueOf(string(inner.Interface().([]byte))))
	}

	return nil
}
