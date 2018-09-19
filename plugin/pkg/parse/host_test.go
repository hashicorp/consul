package parse

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/transport"
)

func TestHostPortOrFile(t *testing.T) {
	tests := []struct {
		in        string
		expected  string
		shouldErr bool
	}{
		{
			"8.8.8.8",
			"8.8.8.8:53",
			false,
		},
		{
			"8.8.8.8:153",
			"8.8.8.8:153",
			false,
		},
		{
			"/etc/resolv.conf:53",
			"",
			true,
		},
		{
			"resolv.conf",
			"127.0.0.1:53",
			false,
		},
	}

	err := ioutil.WriteFile("resolv.conf", []byte("nameserver 127.0.0.1\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to write test resolv.conf")
	}
	defer os.Remove("resolv.conf")

	for i, tc := range tests {
		got, err := HostPortOrFile(tc.in)
		if err == nil && tc.shouldErr {
			t.Errorf("Test %d, expected error, got nil", i)
			continue
		}
		if err != nil && tc.shouldErr {
			continue
		}
		if got[0] != tc.expected {
			t.Errorf("Test %d, expected %q, got %q", i, tc.expected, got[0])
		}
	}
}

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		in        string
		expected  string
		shouldErr bool
	}{
		{"8.8.8.8:53", "8.8.8.8:53", false},
		{"a.a.a.a:153", "", true},
		{"8.8.8.8", "8.8.8.8:53", false},
		{"8.8.8.8:", "8.8.8.8:53", false},
		{"8.8.8.8::53", "", true},
		{"resolv.conf", "", true},
	}

	for i, tc := range tests {
		got, err := HostPort(tc.in, transport.Port)
		if err == nil && tc.shouldErr {
			t.Errorf("Test %d, expected error, got nil", i)
			continue
		}
		if err != nil && !tc.shouldErr {
			t.Errorf("Test %d, expected no error, got %q", i, err)
		}
		if got != tc.expected {
			t.Errorf("Test %d, expected %q, got %q", i, tc.expected, got)
		}
	}
}
