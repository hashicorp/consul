package lib

import (
	"fmt"
)

func PatchSliceOfMaps(m map[string]interface{}, skip []string, skipTree []string) map[string]interface{} {
	return patchValue("", m, skip, skipTree).(map[string]interface{})
}

func patchValue(name string, v interface{}, skip []string, skipTree []string) interface{} {
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
			mm[k] = patchValue(key, v, skip, skipTree)
		}
		return mm

	case []interface{}:
		if len(x) == 0 {
			return nil
		}
		if strSliceContains(name, skipTree) {
			return x
		}
		if strSliceContains(name, skip) {
			for i, y := range x {
				x[i] = patchValue(name, y, skip, skipTree)
			}
			return x
		}
		if _, ok := x[0].(map[string]interface{}); !ok {
			return x
		}
		if len(x) > 1 {
			panic(fmt.Sprintf("%s: []map[string]interface{} with more than one element not supported: %s", name, v))
		}
		return patchValue(name, x[0], skip, skipTree)

	case []map[string]interface{}:
		if len(x) == 0 {
			return nil
		}
		if strSliceContains(name, skipTree) {
			return x
		}
		if strSliceContains(name, skip) {
			for i, y := range x {
				x[i] = patchValue(name, y, skip, skipTree).(map[string]interface{})
			}
			return x
		}
		if len(x) > 1 {
			panic(fmt.Sprintf("%s: []map[string]interface{} with more than one element not supported: %s", name, v))
		}
		return patchValue(name, x[0], skip, skipTree)

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
