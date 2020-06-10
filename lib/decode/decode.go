/*
Package decode provides tools for customizing the decoding of configuration,
into structures using mapstructure.
*/
package decode

import (
	"reflect"
	"strings"
)

// HookTranslateKeys is a mapstructure decode hook which translates keys in a
// map to their canonical value.
//
// Any struct field with a field tag of `alias` may be loaded from any of the
// values keyed by any of the aliases. A field may have one or more alias.
// Aliases must be lowercase, as keys are compared case-insensitive.
//
// Example alias tag:
//    MyField []string `alias:"old_field_name,otherfieldname"`
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
	for k, v := range source {
		lowerK := strings.ToLower(k)
		canonKey, ok := rules[lowerK]
		if !ok {
			continue
		}
		delete(source, k)

		// if there is a value for the canonical key then keep it
		if _, ok := source[canonKey]; ok {
			continue
		}
		source[canonKey] = v
	}
	return source, nil
}

// TODO: could be cached if it is too slow
func translationsForType(to reflect.Type) map[string]string {
	translations := map[string]string{}
	for i := 0; i < to.NumField(); i++ {
		field := to.Field(i)
		canonKey := strings.ToLower(canonicalFieldKey(field))

		tag, ok := field.Tag.Lookup("alias")
		if ok {
			for _, alias := range strings.Split(tag, ",") {
				translations[strings.ToLower(alias)] = canonKey
			}
		}

		// The original key should always be valid too
		lowerName := strings.ToLower(field.Name)
		if canonKey != lowerName {
			translations[lowerName] = canonKey
		}
	}
	return translations
}

func canonicalFieldKey(field reflect.StructField) string {
	tag, ok := field.Tag.Lookup("mapstructure")
	if !ok {
		return field.Name
	}
	parts := strings.SplitN(tag, ",", 2)
	switch {
	case len(parts) < 1:
		return field.Name
	case parts[0] == "":
		return field.Name
	}
	return parts[0]
}

// HookWeakDecodeFromSlice looks for []map[string]interface{} in the source
// data. If the target is not a slice or array it attempts to unpack 1 item
// out of the slice. If there are more items the source data is left unmodified,
// allowing mapstructure to handle and report the decode error caused by
// mismatched types.
//
// If this hook is being used on a "second pass" decode to decode an opaque
// configuration into a type, the DecodeConfig should set WeaklyTypedInput=true,
// (or another hook) to convert any scalar values into a slice of one value when
// the target is a slice. This is necessary because this hook would have converted
// the initial slices into single values on the first pass.
//
// Background
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
		case len(d) == 0:
			return nil, nil
		case len(d) == 1:
			return d[0], nil
		default:
			return data, nil
		}
	default:
		return data, nil
	}
}
