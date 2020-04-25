package lib

import (
	"fmt"
	"strings"
)

func PatchSliceOfMaps(m map[string]interface{}, skip []string, skipTree []string) map[string]interface{} {
	lowerSkip := make([]string, len(skip))
	lowerSkipTree := make([]string, len(skipTree))

	for i, val := range skip {
		lowerSkip[i] = strings.ToLower(val)
	}

	for i, val := range skipTree {
		lowerSkipTree[i] = strings.ToLower(val)
	}

	return patchValue("", m, lowerSkip, lowerSkipTree).(map[string]interface{})
}

func patchValue(name string, v interface{}, skip []string, skipTree []string) interface{} {
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
	lower := strings.ToLower(s)
	for _, vv := range v {
		if lower == vv {
			return true
		}
	}
	return false
}
