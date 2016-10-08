package rcode

import (
	"testing"

	"github.com/miekg/dns"
)

func TestToString(t *testing.T) {
	tests := []struct {
		in       int
		expected string
	}{
		{
			dns.RcodeSuccess,
			"NOERROR",
		},
		{
			28,
			"RCODE28",
		},
	}
	for i, test := range tests {
		got := ToString(test.in)
		if got != test.expected {
			t.Errorf("Test %d, expected %s, got %s", i, test.expected, got)
		}
	}
}
