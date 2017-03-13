package dnsserver

import "testing"

func TestNormalizeZone(t *testing.T) {
	for i, test := range []struct {
		input     string
		expected  string
		shouldErr bool
	}{
		{".", "dns://.:53", false},
		{".:54", "dns://.:54", false},
		{"..", "://:", true},
		{"..", "://:", true},
	} {
		addr, err := normalizeZone(test.input)
		actual := addr.String()
		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error, but there wasn't any", i)
		}
		if !test.shouldErr && err != nil {
			t.Errorf("Test %d: Expected no error, but there was one: %v", i, err)
		}
		if actual != test.expected {
			t.Errorf("Test %d: Expected %s but got %s", i, test.expected, actual)
		}
	}
}
