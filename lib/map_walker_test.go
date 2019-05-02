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
		"simple": tcase{
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
		"uint8 conversion": tcase{
			input: map[string]interface{}{
				"foo": []uint8("bar"),
			},
			expected: map[string]interface{}{
				"foo": []uint8("bar"),
			},
			unexpected: true,
		},
		// TODO(banks): despite the doc comment, MapWalker doesn't actually fix
		// these cases yet. Do that in a later PR.
		// "map iface": tcase{
		//  input: map[string]interface{}{
		//    "foo": map[interface{}]interface{}{
		//      "bar": "baz",
		//    },
		//  },
		//  expected: map[string]interface{}{
		//    "foo": map[string]interface{}{
		//      "bar": "baz",
		//    },
		//  },
		// },
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
