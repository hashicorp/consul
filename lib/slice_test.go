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

func TestStringSliceMergeSorted(t *testing.T) {
	for name, tc := range map[string]struct {
		a, b   []string
		expect []string
	}{
		"nil":              {nil, nil, nil},
		"empty":            {[]string{}, []string{}, nil},
		"one and none":     {[]string{"foo"}, []string{}, []string{"foo"}},
		"one and one dupe": {[]string{"foo"}, []string{"foo"}, []string{"foo"}},
		"one and one":      {[]string{"foo"}, []string{"bar"}, []string{"bar", "foo"}},
		"two and one":      {[]string{"baz", "foo"}, []string{"bar"}, []string{"bar", "baz", "foo"}},
		"two and two":      {[]string{"baz", "foo"}, []string{"bar", "egg"}, []string{"bar", "baz", "egg", "foo"}},
		"two and two dupe": {[]string{"bar", "foo"}, []string{"bar", "egg"}, []string{"bar", "egg", "foo"}},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expect, StringSliceMergeSorted(tc.a, tc.b))
			require.Equal(t, tc.expect, StringSliceMergeSorted(tc.b, tc.a))
		})
	}
}
