// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lib

import (
	"strings"
)

// TranslateKeys recursively translates all keys from m in-place to their
// canonical form as defined in dict which maps an alias name to the canonical
// name. If m already has a value for the canonical name then that one is used
// and the value for the alias name is discarded. Alias names are matched
// case-insensitive.
//
// Example:
//
//	m = TranslateKeys(m, map[string]string{"snake_case": "CamelCase"})
//
// If the canonical string provided is the empty string, the effect is to stop
// recursing into any key matching the left hand side. In this case the left
// hand side must use periods to specify a full path e.g.
// `connect.proxy.config`. The path must be the canonical key names (i.e.
// CamelCase) AFTER translation so NodeName not node_name. These are still match
// in a case-insensitive way.
//
// This is needed for example because parts of the Service Definition are
// "opaque" maps of metadata or config passed to another process or component.
// If we allow translation to recurse we might mangle the "opaque" keys given
// where the clash with key names in other parts of the definition :sob:
//
// Example:
//
//	m - TranslateKeys(m, map[string]string{
//	  "foo_bar": "FooBar",
//	  "widget.config": "",
//	  // Assume widgets is an array, this will prevent recursing into any
//	  // item's config field
//	  "widgets.config": "",
//	})
//
// Deprecated: Use lib/decode.HookTranslateKeys instead.
func TranslateKeys(v map[string]interface{}, dict map[string]string) {
	// Convert all dict keys for exclusions to lower. so we can match against them
	// unambiguously with a single lookup.
	for k, v := range dict {
		if v == "" {
			dict[strings.ToLower(k)] = ""
		}
	}
	ck(v, dict, "")
}

func ck(v interface{}, dict map[string]string, pathPfx string) interface{} {
	// In array case we don't add a path segment for the item as they are all
	// assumed to be same which is why we check the prefix doesn't already end in
	// a .
	if pathPfx != "" && !strings.HasSuffix(pathPfx, ".") {
		pathPfx += "."
	}
	switch x := v.(type) {
	case map[string]interface{}:
		for k, v := range x {
			lowerK := strings.ToLower(k)

			// Check if this path has been excluded
			val, ok := dict[pathPfx+lowerK]
			if ok && val == "" {
				// Don't recurse into this key
				continue
			}

			canonKey, ok := dict[lowerK]

			// no canonical key? -> use this key
			if !ok {
				x[k] = ck(v, dict, pathPfx+lowerK)
				continue
			}

			// delete the alias
			delete(x, k)

			// if there is a value for the canonical key then keep it
			if _, ok := x[canonKey]; ok {
				continue
			}

			// otherwise translate to the canonical key
			x[canonKey] = ck(v, dict, pathPfx+strings.ToLower(canonKey))
		}
		return x

	case []interface{}:
		var a []interface{}
		for _, xv := range x {
			a = append(a, ck(xv, dict, pathPfx))
		}
		return a

	default:
		return v
	}
}
