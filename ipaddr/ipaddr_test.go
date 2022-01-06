package ipaddr

import (
	"fmt"
	"github.com/stretchr/testify/assert"
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

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		address  string
		host     string
		port     int
		hasError bool
	}{
		{"example.org", "example.org", -1, false},
		{"example.org:10", "example.org", 10, false},
		{"example.org:10:10", "", -1, true},

		// IPv4 addresses
		{"10.0.0.1", "10.0.0.1", -1, false},
		{"10.0.0.1:10", "10.0.0.1", 10, false},
		{"10.0.0.1:10:4", "", -1, true},

		// IPv6 addresses w/o port
		{"::1", "::1", -1, false},
		{"fe80::1", "fe80::1", -1, false},
		{"2001:db8::1", "2001:db8::1", -1, false},
		{"2001:db8:1:2:3:4:5:6:7:8", "", -1, true},
		{"2a05:d014:d9e:c303:e4d3:d281:a61d:8ebd", "2a05:d014:d9e:c303:e4d3:d281:a61d:8ebd", -1, false},

		{"[::ffff:172.16.5.4]", "::ffff:172.16.5.4", -1, false},

		// IPv6 addresses with port
		{"[::1]:10", "::1", 10, false},
		{"[fe80::1]:10", "fe80::1", 10, false},
		{"[2001:db8::1]:10", "2001:db8::1", 10, false},
		// Wierd IPv6 address with port
		{"2001:db8:1:2:3:4:5:6:7", "2001:db8:1:2:3:4:5:6", 7, false},
	}
	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			host, port, err := SplitHostPort(tt.address)
			if tt.hasError {
				assert.NotNil(t, err)
			} else {

				assert.Equal(t, tt.host, host)
				assert.Equal(t, tt.port, port)
				assert.Nil(t, err)
			}
		})
	}
}
