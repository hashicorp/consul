package kubernetes

import (
	"net"
	"testing"
)

func TestIsRequestInReverseRange(t *testing.T) {

	tests := []struct {
		cidr     string
		name     string
		expected bool
	}{
		{"1.2.3.0/24", "4.3.2.1.in-addr.arpa.", true},
		{"1.2.3.0/24", "5.3.2.1.in-addr.arpa.", true},
		{"1.2.3.0/24", "5.4.2.1.in-addr.arpa.", false},
		{"5.6.0.0/16", "5.4.2.1.in-addr.arpa.", false},
		{"5.6.0.0/16", "5.4.6.5.in-addr.arpa.", true},
		{"5.6.0.0/16", "5.6.0.1.in-addr.arpa.", false},
	}

	k := Kubernetes{Zones: []string{"inter.webs.test"}}

	for _, test := range tests {
		_, cidr, _ := net.ParseCIDR(test.cidr)
		k.ReverseCidrs = []net.IPNet{*cidr}
		result := k.isRequestInReverseRange(test.name)
		if result != test.expected {
			t.Errorf("Expected '%v' for '%v' in %v.", test.expected, test.name, test.cidr)
		}
	}
}
