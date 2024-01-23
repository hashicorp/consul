// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dnsutil

import (
	"testing"

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
			require.False(t, IsValidLabel("*"))
		})
		t.Run("ValidateLabel", func(t *testing.T) {
			require.Error(t, ValidateLabel("*"))
		})
	})

	for name, expect := range cases {
		t.Run(name, func(t *testing.T) {
			t.Run("IsValidDNSLabel", func(t *testing.T) {
				require.Equal(t, expect, IsValidLabel(name))
			})
			t.Run("ValidateLabel", func(t *testing.T) {
				if expect {
					require.NoError(t, ValidateLabel(name))
				} else {
					require.Error(t, ValidateLabel(name))
				}
			})
		})
	}
}

func TestDNSInvalidRegex(t *testing.T) {
	tests := []struct {
		desc    string
		in      string
		invalid bool
	}{
		{"Valid Hostname", "testnode", false},
		{"Valid Hostname", "test-node", false},
		{"Invalid Hostname with special chars", "test#$$!node", true},
		{"Invalid Hostname with special chars in the end", "testnode%^", true},
		{"Whitespace", "  ", true},
		{"Only special chars", "./$", true},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if got, want := InvalidNameRe.MatchString(test.in), test.invalid; got != want {
				t.Fatalf("Expected %v to return %v", test.in, want)
			}
		})

	}
}
