// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ipaddr

import (
	"fmt"
	"testing"
)

func TestIsIPv6(t *testing.T) {
	tests := []struct {
		ip   string
		ipv6 bool
	}{
		// IPv4 private addresses
		{"10.0.0.1", false},    // private network address
		{"100.64.0.1", false},  // shared address space
		{"172.16.0.1", false},  // private network address
		{"192.168.0.1", false}, // private network address
		{"192.0.0.1", false},   // IANA address
		{"192.0.2.1", false},   // documentation address
		{"127.0.0.1", false},   // loopback address
		{"169.254.0.1", false}, // link local address

		// IPv4 public addresses
		{"1.2.3.4", false},

		// IPv6 private addresses
		{"::1", true},         // loopback address
		{"fe80::1", true},     // link local address
		{"fc00::1", true},     // unique local address
		{"fec0::1", true},     // site local address
		{"2001:db8::1", true}, // documentation address

		// IPv6 public addresses
		{"2004:db6::1", true},

		// hostname
		{"example.com", false},
		{"localhost", false},
		{"1.257.0.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			port := 1234
			formated := FormatAddressPort(tt.ip, port)
			if tt.ipv6 {
				if fmt.Sprintf("[%s]:%d", tt.ip, port) != formated {
					t.Fatalf("Wrong format %s for %s", formated, tt.ip)
				}
			} else {
				if fmt.Sprintf("%s:%d", tt.ip, port) != formated {
					t.Fatalf("Wrong format %s for %s", formated, tt.ip)
				}
			}
		})
	}
}

// Copied from https://github.com/hashicorp/memberlist/blob/master/util_test.go
func TestUtil_PortFunctions(t *testing.T) {
	tests := []struct {
		addr       string
		hasPort    bool
		ensurePort string
	}{
		{"1.2.3.4", false, "1.2.3.4:8301"},
		{"1.2.3.4:1234", true, "1.2.3.4:1234"},
		{"2600:1f14:e22:1501:f9a:2e0c:a167:67e8", false, "[2600:1f14:e22:1501:f9a:2e0c:a167:67e8]:8301"},
		{"[2600:1f14:e22:1501:f9a:2e0c:a167:67e8]", false, "[2600:1f14:e22:1501:f9a:2e0c:a167:67e8]:8301"},
		{"[2600:1f14:e22:1501:f9a:2e0c:a167:67e8]:1234", true, "[2600:1f14:e22:1501:f9a:2e0c:a167:67e8]:1234"},
		{"localhost", false, "localhost:8301"},
		{"localhost:1234", true, "localhost:1234"},
		{"hashicorp.com", false, "hashicorp.com:8301"},
		{"hashicorp.com:1234", true, "hashicorp.com:1234"},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if got, want := HasPort(tt.addr), tt.hasPort; got != want {
				t.Fatalf("got %v want %v", got, want)
			}
			if got, want := EnsurePort(tt.addr, 8301), tt.ensurePort; got != want {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
}
