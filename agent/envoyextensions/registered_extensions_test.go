package envoyextensions

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestValidateExtensions(t *testing.T) {
	tests := map[string]struct {
		input      []api.EnvoyExtension
		expectErrs []string
	}{
		"missing name": {
			input:      []api.EnvoyExtension{{}},
			expectErrs: []string{"Name is required"},
		},
		"bad name": {
			input: []api.EnvoyExtension{{
				Name: "bad",
			}},
			expectErrs: []string{"not a built-in extension"},
		},
		"multiple errors": {
			input: []api.EnvoyExtension{
				{},
				{
					Name: "bad",
				},
			},
			expectErrs: []string{
				"invalid EnvoyExtensions[0]: Name is required",
				"invalid EnvoyExtensions[1][bad]:",
			},
		},
		"invalid arguments to constructor": {
			input: []api.EnvoyExtension{{
				Name: "builtin/lua",
			}},
			expectErrs: []string{
				"invalid EnvoyExtensions[0][builtin/lua]",
				"missing Script value",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateExtensions(tc.input)
			if len(tc.expectErrs) == 0 {
				require.NoError(t, err)
				return
			}
			for _, e := range tc.expectErrs {
				require.ErrorContains(t, err, e)
			}
		})
	}
}
