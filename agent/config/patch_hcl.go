package config

import (
	"fmt"
)

func patchSliceOfMaps(m map[string]interface{}, skip []string) map[string]interface{} {
	return patchValue("", m, skip).(map[string]interface{})
}

func patchValue(name string, v interface{}, skip []string) interface{} {
	// fmt.Printf("%q: %T\n", name, v)
	switch x := v.(type) {
	case map[string]interface{}:
		if len(x) == 0 {
			return x
		}
		mm := make(map[string]interface{})
		for k, v := range x {
			key := k
			if name != "" {
				key = name + "." + k
			}
			mm[k] = patchValue(key, v, skip)
		}
		return mm

	case []interface{}:
		if len(x) == 0 {
			return nil
		}
		if strSliceContains(name, skip) {
			for i, y := range x {
				x[i] = patchValue(name, y, skip)
			}
			return x
		}
		if _, ok := x[0].(map[string]interface{}); !ok {
			return x
		}
		if len(x) > 1 {
			panic(fmt.Sprintf("%s: []map[string]interface{} with more than one element not supported: %s", name, v))
		}
		return patchValue(name, x[0], skip)

	case []map[string]interface{}:
		if len(x) == 0 {
			return nil
		}
		if strSliceContains(name, skip) {
			for i, y := range x {
				x[i] = patchValue(name, y, skip).(map[string]interface{})
			}
			return x
		}
		if len(x) > 1 {
			panic(fmt.Sprintf("%s: []map[string]interface{} with more than one element not supported: %s", name, v))
		}
		return patchValue(name, x[0], skip)

	default:
		return v
	}
}

func strSliceContains(s string, v []string) bool {
	for _, vv := range v {
		if s == vv {
			return true
		}
	}
	return false
}
