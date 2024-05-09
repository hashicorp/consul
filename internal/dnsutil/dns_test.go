// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dnsutil

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

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
