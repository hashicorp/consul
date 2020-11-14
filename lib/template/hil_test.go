package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterpolateHIL(t *testing.T) {
	for name, test := range map[string]struct {
		in       string
		vars     map[string]string
		exp      string // when lower=false
		expLower string // when lower=true
		ok       bool
	}{
		// valid HIL
		"empty": {
			"",
			map[string]string{},
			"",
			"",
			true,
		},
		"no vars": {
			"nothing",
			map[string]string{},
			"nothing",
			"nothing",
			true,
		},
		"just lowercase var": {
			"${item}",
			map[string]string{"item": "value"},
			"value",
			"value",
			true,
		},
		"just uppercase var": {
			"${item}",
			map[string]string{"item": "VaLuE"},
			"VaLuE",
			"value",
			true,
		},
		"lowercase var in middle": {
			"before ${item}after",
			map[string]string{"item": "value"},
			"before valueafter",
			"before valueafter",
			true,
		},
		"uppercase var in middle": {
			"before ${item}after",
			map[string]string{"item": "VaLuE"},
			"before VaLuEafter",
			"before valueafter",
			true,
		},
		"two vars": {
			"before ${item}after ${more}",
			map[string]string{"item": "value", "more": "xyz"},
			"before valueafter xyz",
			"before valueafter xyz",
			true,
		},
		"missing map val": {
			"${item}",
			map[string]string{"item": ""},
			"",
			"",
			true,
		},
		// "weird" HIL, but not technically invalid
		"just end": {
			"}",
			map[string]string{},
			"}",
			"}",
			true,
		},
		"var without start": {
			" item }",
			map[string]string{"item": "value"},
			" item }",
			" item }",
			true,
		},
		"two vars missing second start": {
			"before ${ item }after  more }",
			map[string]string{"item": "value", "more": "xyz"},
			"before valueafter  more }",
			"before valueafter  more }",
			true,
		},
		// invalid HIL
		"just start": {
			"${",
			map[string]string{},
			"",
			"",
			false,
		},
		"backwards": {
			"}${",
			map[string]string{},
			"",
			"",
			false,
		},
		"no varname": {
			"${}",
			map[string]string{},
			"",
			"",
			false,
		},
		"missing map key": {
			"${item}",
			map[string]string{},
			"",
			"",
			false,
		},
		"var without end": {
			"${ item ",
			map[string]string{"item": "value"},
			"",
			"",
			false,
		},
		"two vars missing first end": {
			"before ${ item after ${ more }",
			map[string]string{"item": "value", "more": "xyz"},
			"",
			"",
			false,
		},
	} {
		test := test
		t.Run(name+" lower=false", func(t *testing.T) {
			out, err := InterpolateHIL(test.in, test.vars, false)
			if test.ok {
				require.NoError(t, err)
				require.Equal(t, test.exp, out)
			} else {
				require.NotNil(t, err)
				require.Equal(t, out, "")
			}
		})
		t.Run(name+" lower=true", func(t *testing.T) {
			out, err := InterpolateHIL(test.in, test.vars, true)
			if test.ok {
				require.NoError(t, err)
				require.Equal(t, test.expLower, out)
			} else {
				require.NotNil(t, err)
				require.Equal(t, out, "")
			}
		})
	}
}
