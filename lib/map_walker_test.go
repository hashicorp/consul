// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapWalk(t *testing.T) {
	t.Parallel()
	type tcase struct {
		input      interface{}
		expected   interface{}
		unexpected bool
		err        bool
	}

	cases := map[string]tcase{
		// basically tests that []uint8 gets turned into
		// a string
		"simple": {
			input: map[string]interface{}{
				"foo": []uint8("bar"),
			},
			expected: map[string]interface{}{
				"foo": "bar",
			},
		},
		// ensures that it was actually converted and not
		// just the require.Equal masking the underlying
		// type differences
		"uint8 conversion": {
			input: map[string]interface{}{
				"foo": []uint8("bar"),
			},
			expected: map[string]interface{}{
				"foo": []uint8("bar"),
			},
			unexpected: true,
		},
		// ensure we don't panic from trying to call reflect.Value.Type
		// on a nil pointer
		"nil pointer": {
			input: map[string]interface{}{
				"foo": nil,
			},
			expected: map[string]interface{}{
				"foo": nil,
			},
		},
		// ensure nested maps get processed correctly
		"nested": {
			input: map[string]interface{}{
				"foo": map[interface{}]interface{}{
					"bar": []uint8("baz"),
				},
				"bar": []uint8("baz"),
			},
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
				"bar": "baz",
			},
		},
		"map with slice": {
			input: map[string]interface{}{
				"foo": []uint8("bar"),
				"bar": []interface{}{
					[]uint8("one"),
					[]uint8("two"),
					3,
					4,
				},
			},
			expected: map[string]interface{}{
				"foo": "bar",
				"bar": []interface{}{
					"one",
					"two",
					3,
					4,
				},
			},
		},
	}

	for name, tcase := range cases {
		name := name
		tcase := tcase

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual, err := MapWalk(tcase.input)
			if tcase.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tcase.unexpected {
					require.NotEqual(t, tcase.expected, actual)
				} else {
					require.Equal(t, tcase.expected, actual)
				}
			}
		})
	}
}
