package strutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrListContains(t *testing.T) {
	tests := []struct {
		haystack []string
		needle   string
		expected bool
	}{
		// found
		{[]string{"a"}, "a", true},
		{[]string{"a", "b", "c"}, "a", true},
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "c", true},

		// not found
		{nil, "", false},
		{[]string{}, "", false},
		{[]string{"a"}, "", false},
		{[]string{"a"}, "b", false},
		{[]string{"a", "b", "c"}, "x", false},
	}
	for _, test := range tests {
		ok := StrListContains(test.haystack, test.needle)
		assert.Equal(t, test.expected, ok, "failed on %s/%v", test.needle, test.haystack)
	}
}
