// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

/*
Package decode provides tools for customizing the decoding of configuration,
into structures using mapstructure.
*/
package decode

import (
	"reflect"
	"strings"

	"github.com/mitchellh/reflectwalk"
)

// HookTranslateKeys is a mapstructure decode hook which translates keys in a
// map to their canonical value.
//
// Any struct field with a field tag of `alias` may be loaded from any of the
// values keyed by any of the aliases. A field may have one or more alias.
// Aliases must be lowercase, as keys are compared case-insensitive.
//
// Example alias tag:
//
//	MyField []string `alias:"old_field_name,otherfieldname"`
//
// This hook should ONLY be used to maintain backwards compatibility with
// deprecated keys. For new structures use mapstructure struct tags to set the
// desired serialization key.
//
// IMPORTANT: This function assumes that mapstructure is being used with the
// default struct field tag of `mapstructure`. If mapstructure.DecoderConfig.TagName
// is set to a different value this function will need to be parameterized with
// that value to correctly find the canonical data key.
func HookTranslateKeys(_, to reflect.Type, data interface{}) (interface{}, error) {
	// Return immediately if target is not a struct, as only structs can have
	// field tags. If the target is a pointer to a struct, mapstructure will call
	// the hook again with the struct.
	if to.Kind() != reflect.Struct {
		return data, nil
	}

	// Avoid doing any work if data is not a map
	source, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	rules := translationsForType(to)
	// Avoid making a copy if there are no translation rules
	if len(rules) == 0 {
		return data, nil
	}
	result := make(map[string]interface{}, len(source))
	for k, v := range source {
		lowerK := strings.ToLower(k)
		canonKey, ok := rules[lowerK]
		if !ok {
			result[k] = v
			continue
		}

		// if there is a value for the canonical key then keep it
		if canonValue, ok := source[canonKey]; ok {
			// Assign the value for the case where canonKey == k
			result[canonKey] = canonValue
			continue
		}
		result[canonKey] = v
	}
	return result, nil
}

// TODO: could be cached if it is too slow
func translationsForType(to reflect.Type) map[string]string {
	translations := map[string]string{}
	for i := 0; i < to.NumField(); i++ {
		field := to.Field(i)
		tags := fieldTags(field)
		if tags.squash {
			embedded := field.Type
			if embedded.Kind() == reflect.Ptr {
				embedded = embedded.Elem()
			}
			if embedded.Kind() != reflect.Struct {
				// mapstructure will handle reporting this error
				continue
			}

			for k, v := range translationsForType(embedded) {
				translations[k] = v
			}
			continue
		}

		tag, ok := field.Tag.Lookup("alias")
		if !ok {
			continue
		}
		canonKey := strings.ToLower(tags.name)
		for _, alias := range strings.Split(tag, ",") {
			translations[strings.ToLower(alias)] = canonKey
		}
	}
	return translations
}

func fieldTags(field reflect.StructField) mapstructureFieldTags {
	tag, ok := field.Tag.Lookup("mapstructure")
	if !ok {
		return mapstructureFieldTags{name: field.Name}
	}

	tags := mapstructureFieldTags{name: field.Name}
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return tags
	}
	if parts[0] != "" {
		tags.name = parts[0]
	}
	for _, part := range parts[1:] {
		if part == "squash" {
			tags.squash = true
		}
	}
	return tags
}

type mapstructureFieldTags struct {
	name   string
	squash bool
}

// HookWeakDecodeFromSlice looks for []map[string]interface{} and []interface{}
// in the source data. If the target is not a slice or array it attempts to unpack
// 1 item out of the slice. If there are more items the source data is left
// unmodified, allowing mapstructure to handle and report the decode error caused by
// mismatched types. The []interface{} is handled so that all slice types are
// behave the same way, and for the rare case when a raw structure is re-encoded
// to JSON, which will produce the []interface{}.
//
// If this hook is being used on a "second pass" decode to decode an opaque
// configuration into a type, the DecodeConfig should set WeaklyTypedInput=true,
// (or another hook) to convert any scalar values into a slice of one value when
// the target is a slice. This is necessary because this hook would have converted
// the initial slices into single values on the first pass.
//
// # Background
//
// HCL allows for repeated blocks which forces it to store structures
// as []map[string]interface{} instead of map[string]interface{}. This is an
// ambiguity which makes the generated structures incompatible with the
// corresponding JSON data.
//
// This hook allows config to be read from the HCL format into a raw structure,
// and later decoded into a strongly typed structure.
func HookWeakDecodeFromSlice(from, to reflect.Type, data interface{}) (interface{}, error) {
	if from.Kind() == reflect.Slice && (to.Kind() == reflect.Slice || to.Kind() == reflect.Array) {
		return data, nil
	}

	switch d := data.(type) {
	case []map[string]interface{}:
		switch {
		case len(d) != 1:
			return data, nil
		case to == typeOfEmptyInterface:
			return unSlice(d[0])
		default:
			return d[0], nil
		}

	// a slice map be encoded as []interface{} in some cases
	case []interface{}:
		switch {
		case len(d) != 1:
			return data, nil
		case to == typeOfEmptyInterface:
			return unSlice(d[0])
		default:
			return d[0], nil
		}
	}
	return data, nil
}

var typeOfEmptyInterface = reflect.TypeOf((*interface{})(nil)).Elem()

func unSlice(data interface{}) (interface{}, error) {
	err := reflectwalk.Walk(data, &unSliceWalker{})
	return data, err
}

type unSliceWalker struct{}

func (u *unSliceWalker) Map(_ reflect.Value) error {
	return nil
}

func (u *unSliceWalker) MapElem(m, k, v reflect.Value) error {
	if !v.IsValid() || v.Kind() != reflect.Interface {
		return nil
	}

	v = v.Elem() // unpack the value from the interface{}
	if v.Kind() != reflect.Slice || v.Len() != 1 {
		return nil
	}

	first := v.Index(0)
	// The value should always be assignable, but double check to avoid a panic.
	if !first.Type().AssignableTo(m.Type().Elem()) {
		return nil
	}
	m.SetMapIndex(k, first)
	return nil
}
