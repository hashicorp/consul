// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ipaddr

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		// IPv4 private addresses
		{"10.0.0.1", true},    // private network address
		{"100.64.0.1", true},  // shared address space
		{"172.16.0.1", true},  // private network address
		{"192.168.0.1", true}, // private network address
		{"192.0.0.1", true},   // IANA address
		{"192.0.2.1", true},   // documentation address
		{"127.0.0.1", true},   // loopback address
		{"169.254.0.1", true}, // link local address

		// IPv4 public addresses
		{"1.2.3.4", false},

		// IPv6 private addresses
		{"::1", true},         // loopback address
		{"fe80::1", true},     // link local address
		{"fc00::1", true},     // unique local address
		{"fec0::1", true},     // site local address
		{"2001:db8::1", true}, // documentation address

		// IPv6 public addresses
		{"2004:db6::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("%s is not a valid ip address", tt.ip)
			}
			if got, want := isPrivate(ip), tt.private; got != want {
				t.Fatalf("got %v for %v want %v", got, ip, want)
			}
		})
	}
}
