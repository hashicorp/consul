package lib

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringSliceEqual(t *testing.T) {
	for _, tc := range []struct {
		a, b  []string
		equal bool
	}{
		{nil, nil, true},
		{nil, []string{}, true},
		{[]string{}, []string{}, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{}, []string{"a"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, false},
	} {
		name := fmt.Sprintf("%#v =?= %#v", tc.a, tc.b)
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.equal, StringSliceEqual(tc.a, tc.b))
			require.Equal(t, tc.equal, StringSliceEqual(tc.b, tc.a))
		})
	}
}
