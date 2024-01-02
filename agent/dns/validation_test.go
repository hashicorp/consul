// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns_test

import (
	"testing"

	"github.com/hashicorp/consul/agent/dns"
	"github.com/stretchr/testify/require"
)

func TestValidLabel(t *testing.T) {
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
		"thisvaluecontainsalotofcharactersbutnottoomanyandthecaseisatrue":  true,  // 63 chars
		"thisvaluecontainstoomanycharactersandisthusinvalidandtestisfalse": false, // 64 chars
	}

	t.Run("*", func(t *testing.T) {
		t.Run("IsValidLabel", func(t *testing.T) {
			require.False(t, dns.IsValidLabel("*"))
		})
		t.Run("ValidateLabel", func(t *testing.T) {
			require.Error(t, dns.ValidateLabel("*"))
		})
	})

	for name, expect := range cases {
		t.Run(name, func(t *testing.T) {
			t.Run("IsValidDNSLabel", func(t *testing.T) {
				require.Equal(t, expect, dns.IsValidLabel(name))
			})
			t.Run("ValidateLabel", func(t *testing.T) {
				if expect {
					require.NoError(t, dns.ValidateLabel(name))
				} else {
					require.Error(t, dns.ValidateLabel(name))
				}
			})
		})
	}
}
