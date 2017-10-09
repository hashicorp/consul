package config

import "strings"

// TranslateKeys recursively translates all keys from m in-place to their
// canonical form as defined in dict which maps an alias name to the canonical
// name. If m already has a value for the canonical name then that one is used
// and the value for the alias name is discarded. Alias names are matched
// case-insensitive.
//
// Example:
//
//   m = TranslateKeys(m, map[string]string{"CamelCase": "snake_case"})
//
func TranslateKeys(v map[string]interface{}, dict map[string]string) {
	ck(v, dict)
}

func ck(v interface{}, dict map[string]string) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, v := range x {
			canonKey := dict[strings.ToLower(k)]

			// no canonical key? -> use this key
			if canonKey == "" {
				x[k] = ck(v, dict)
				continue
			}

			// delete the alias
			delete(x, k)

			// if there is a value for the canonical key then keep it
			if _, ok := x[canonKey]; ok {
				continue
			}

			// otherwise translate to the canonical key
			x[canonKey] = ck(v, dict)
		}
		return x

	case []interface{}:
		var a []interface{}
		for _, xv := range x {
			a = append(a, ck(xv, dict))
		}
		return a

	default:
		return v
	}
}
