package validation_test

import (
	"testing"

	"github.com/hashicorp/consul/lib/validation"
	"github.com/stretchr/testify/require"
)

func TestValidDNSLabel(t *testing.T) {
	cases := map[string]bool{
		"CrEaTeD":           true,
		"created":           true,
		"create-deleted":    true,
		"foo":               true,
		"":                  false,
		"_foo_":             false,
		"-foo":              false,
		"foo-":              false,
		"-foo-":             false,
		"-foo-bar-":         false,
		"no spaces allowed": false,
	}

	t.Run("*", func(t *testing.T) {
		t.Run("IsValidDNSLabel", func(t *testing.T) {
			require.False(t, validation.IsValidDNSLabel("*"))
		})
		t.Run("RequireValidDNSLabel", func(t *testing.T) {
			require.Error(t, validation.RequireValidDNSLabel("*"))
		})
	})

	for name, expect := range cases {
		t.Run(name, func(t *testing.T) {
			t.Run("IsValidDNSLabel", func(t *testing.T) {
				require.Equal(t, expect, validation.IsValidDNSLabel(name))
			})
			t.Run("RequireValidDNSLabel", func(t *testing.T) {
				if expect {
					require.NoError(t, validation.RequireValidDNSLabel(name))
				} else {
					require.Error(t, validation.RequireValidDNSLabel(name))
				}
			})
		})
	}
}
