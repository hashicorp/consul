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
		{".:", "://:", true},
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

func TestNormalizeZoneReverse(t *testing.T) {
	for i, test := range []struct {
		input     string
		expected  string
		shouldErr bool
	}{
		{"2003::1/64", "dns://0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.2.ip6.arpa.:53", false},
		{"2003::1/64.", "dns://2003::1/64.:53", false}, // OK, with closing dot the parse will fail.
		{"2003::1/64:53", "dns://0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.2.ip6.arpa.:53", false},
		{"2003::1/64.:53", "dns://2003::1/64.:53", false},

		{"10.0.0.0/24", "dns://0.0.10.in-addr.arpa.:53", false},
		{"10.0.0.0/24.", "dns://10.0.0.0/24.:53", false},
		{"10.0.0.0/24:53", "dns://0.0.10.in-addr.arpa.:53", false},
		{"10.0.0.0/24.:53", "dns://10.0.0.0/24.:53", false},

		// non %8==0 netmasks
		{"2003::53/67", "dns://0.0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.2.ip6.arpa.:53", false},
		{"10.0.0.0/25.", "dns://10.0.0.0/25.:53", false}, // has dot
		{"10.0.0.0/25", "dns://0.0.10.in-addr.arpa.:53", false},
		{"fd00:77:30::0/110", "dns://0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.7.7.0.0.0.0.d.f.ip6.arpa.:53", false},
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
