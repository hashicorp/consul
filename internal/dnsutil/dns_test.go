// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dnsutil

import (
	"net"
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

func Test_IPFromARPA(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected net.IP
	}{
		{
			name:     "valid ipv4",
			input:    "4.3.2.1.in-addr.arpa.",
			expected: net.ParseIP("1.2.3.4"),
		},
		{
			name:     "valid ipv6",
			input:    "b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
			expected: net.ParseIP("2001:db8::567:89ab"),
		},
		{
			name:  "invalid subdomain",
			input: "4.3.2.1.addressplz.arpa",
		},
		{
			name:  "invalid ipv4 - invalid octet",
			input: "277.3.2.1.in-addr.arpa",
		},
		{
			name:  "invalid ipv4 - too short",
			input: "3.2.1.in-addr.arpa",
		},
		{
			name:  "invalid ipv6 - invalid hex char",
			input: "x.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
		},
		{
			name:  "invalid ipv6 - too long",
			input: "d.b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := IPFromARPA(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}
