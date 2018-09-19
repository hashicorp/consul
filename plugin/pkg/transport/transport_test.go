package transport

import "testing"

func TestParse(t *testing.T) {
	for i, test := range []struct {
		input    string
		expected string
	}{
		{"dns://.:53", DNS},
		{"2003::1/64.:53", DNS},
		{"grpc://example.org:1443 ", GRPC},
		{"tls://example.org ", TLS},
		{"https://example.org ", HTTPS},
	} {
		actual, _ := Parse(test.input)
		if actual != test.expected {
			t.Errorf("Test %d: Expected %s but got %s", i, test.expected, actual)
		}
	}
}
