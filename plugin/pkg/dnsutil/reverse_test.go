package dnsutil

import (
	"testing"
)

func TestExtractAddressFromReverse(t *testing.T) {
	tests := []struct {
		reverseName     string
		expectedAddress string
	}{
		{
			"54.119.58.176.in-addr.arpa.",
			"176.58.119.54",
		},
		{
			".58.176.in-addr.arpa.",
			"",
		},
		{
			"b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.in-addr.arpa.",
			"",
		},
		{
			"b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			"2001:db8::567:89ab",
		},
		{
			"d.0.1.0.0.2.ip6.arpa.",
			"",
		},
		{
			"54.119.58.176.ip6.arpa.",
			"",
		},
		{
			"NONAME",
			"",
		},
		{
			"",
			"",
		},
	}
	for i, test := range tests {
		got := ExtractAddressFromReverse(test.reverseName)
		if got != test.expectedAddress {
			t.Errorf("Test %d, expected '%s', got '%s'", i, test.expectedAddress, got)
		}
	}
}
