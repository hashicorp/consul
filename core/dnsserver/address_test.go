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
		{".:", "://:", true},
		{"dns://.", "dns://.:53", false},
		{"dns://.:5353", "dns://.:5353", false},
		{"dns://..", "://:", true},
		{"dns://.:", "://:", true},
		{"tls://.", "tls://.:853", false},
		{"tls://.:8953", "tls://.:8953", false},
		{"tls://..", "://:", true},
		{"tls://.:", "://:", true},
		{"grpc://.", "grpc://.:443", false},
		{"grpc://.:8443", "grpc://.:8443", false},
		{"grpc://..", "://:", true},
		{"grpc://.:", "://:", true},
		{"https://.", "https://.:443", false},
		{"https://.:8443", "https://.:8443", false},
		{"https://..", "://:", true},
		{"https://.:", "://:", true},
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

func TestSplitProtocolHostPort(t *testing.T) {
	for i, test := range []struct {
		input     string
		proto     string
		ip        string
		port      string
		shouldErr bool
	}{
		{"dns://:53", "dns", "", "53", false},
		{"dns://127.0.0.1:4005", "dns", "127.0.0.1", "4005", false},
		{"[ffe0:34ab:1]:4005", "", "ffe0:34ab:1", "4005", false},

		// port part is mandatory
		{"dns://", "dns", "", "", true},
		{"dns://127.0.0.1", "dns", "127.0.0.1", "", true},
		// cannot be empty
		{"", "", "", "", true},
		// invalid format with twice ://
		{"dns://127.0.0.1://53", "", "", "", true},
	} {
		proto, ip, port, err := SplitProtocolHostPort(test.input)
		if test.shouldErr && err == nil {
			t.Errorf("Test %d: (address = %s) expected error, but there wasn't any", i, test.input)
			continue
		}
		if !test.shouldErr && err != nil {
			t.Errorf("Test %d: (address = %s) expected no error, but there was one: %v", i, test.input, err)
			continue
		}
		if err == nil || test.shouldErr {
			continue
		}
		if proto != test.proto {
			t.Errorf("Test %d: (address = %s) expected protocol with value %s but got %s", i, test.input, test.proto, proto)
		}
		if ip != test.ip {
			t.Errorf("Test %d: (address = %s) expected ip with value %s but got %s", i, test.input, test.ip, ip)
		}
		if port != test.port {
			t.Errorf("Test %d: (address = %s) expected port with value %s but got %s", i, test.input, test.port, port)
		}

	}
}

type checkCall struct {
	zone       zoneAddr
	same       bool
	overlap    bool
	overlapKey string
}

type checkTest struct {
	sequence []checkCall
}

func TestOverlapAddressChecker(t *testing.T) {
	for i, test := range []checkTest{
		{sequence: []checkCall{
			{zoneAddr{Transport: "dns", Zone: ".", Address: "", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "", Port: "53"}, true, false, ""},
		},
		},
		{sequence: []checkCall{
			{zoneAddr{Transport: "dns", Zone: ".", Address: "", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "", Port: "54"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "127.0.0.1", Port: "53"}, false, true, "dns://.:53"},
		},
		},
		{sequence: []checkCall{
			{zoneAddr{Transport: "dns", Zone: ".", Address: "127.0.0.1", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "", Port: "54"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "127.0.0.1", Port: "53"}, true, false, ""},
		},
		},
		{sequence: []checkCall{
			{zoneAddr{Transport: "dns", Zone: ".", Address: "127.0.0.1", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "", Port: "54"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "128.0.0.1", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "129.0.0.1", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: ".", Address: "", Port: "53"}, false, true, "dns://.:53 on 129.0.0.1"},
		},
		},
		{sequence: []checkCall{
			{zoneAddr{Transport: "dns", Zone: ".", Address: "127.0.0.1", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: "com.", Address: "127.0.0.1", Port: "53"}, false, false, ""},
			{zoneAddr{Transport: "dns", Zone: "com.", Address: "", Port: "53"}, false, true, "dns://com.:53 on 127.0.0.1"},
		},
		},
	} {

		checker := newOverlapZone()
		for _, call := range test.sequence {
			same, overlap := checker.registerAndCheck(call.zone)
			sZone := call.zone.String()
			if (same != nil) != call.same {
				t.Errorf("Test %d: error, for zone %s, 'same' (%v) has not the expected value (%v)", i, sZone, same != nil, call.same)
			}
			if same == nil {
				if (overlap != nil) != call.overlap {
					t.Errorf("Test %d: error, for zone %s, 'overlap' (%v) has not the expected value (%v)", i, sZone, overlap != nil, call.overlap)
				}
				if overlap != nil {
					if overlap.String() != call.overlapKey {
						t.Errorf("Test %d: error, for zone %s, 'overlap Key' (%v) has not the expected value (%v)", i, sZone, overlap.String(), call.overlapKey)
					}

				}
			}

		}
	}
}
