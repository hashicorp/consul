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
		tag, ok := field.Tag.Lookup("alias")
		if !ok {
			continue
		}

		canonKey := strings.ToLower(canonicalFieldKey(field))
		for _, alias := range strings.Split(tag, ",") {
			translations[strings.ToLower(alias)] = canonKey
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
