package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// mapToKV converts a map[string]string into a human-friendly key=value list,
// sorted by name.
func mapToKV(m map[string]string, joiner string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := make([]string, len(keys))
	for i, k := range keys {
		r[i] = fmt.Sprintf("%s=%s", k, m[k])
	}
	return strings.Join(r, joiner)
}
