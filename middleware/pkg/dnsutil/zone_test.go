package dnsutil

import (
	"errors"
	"testing"

	"github.com/miekg/dns"
)

func TestTrimZone(t *testing.T) {
	tests := []struct {
		qname    string
		zone     string
		expected string
		err      error
	}{
		{"a.example.org", "example.org", "a", nil},
		{"a.b.example.org", "example.org", "a.b", nil},
		{"b.", ".", "b", nil},
		{"example.org", "example.org", "", errors.New("should err")},
		{"org", "example.org", "", errors.New("should err")},
	}

	for i, tc := range tests {
		got, err := TrimZone(dns.Fqdn(tc.qname), dns.Fqdn(tc.zone))
		if tc.err != nil && err == nil {
			t.Errorf("Test %d, expected error got nil", i)
			continue
		}
		if tc.err == nil && err != nil {
			t.Errorf("Test %d, expected no error got %v", i, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("Test %d, expected %s, got %s", i, tc.expected, got)
			continue
		}
	}
}
