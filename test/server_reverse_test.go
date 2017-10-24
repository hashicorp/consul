package test

import (
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestClasslessReverse(t *testing.T) {
	// 25 -> so anything above 1.127 won't be answered, below is OK.
	corefile := `192.168.1.0/25:0 {
		whoami
}
`
	s, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer s.Stop()

	tests := []struct {
		addr  string
		rcode int
	}{
		{"192.168.1.0", dns.RcodeSuccess},   // in range
		{"192.168.1.1", dns.RcodeSuccess},   // in range
		{"192.168.1.127", dns.RcodeSuccess}, // in range

		{"192.168.1.128", dns.RcodeRefused}, // out of range
		{"192.168.1.129", dns.RcodeRefused}, // out of range
		{"192.168.1.255", dns.RcodeRefused}, // out of range
		{"192.168.2.0", dns.RcodeRefused},   // different zone
	}

	m := new(dns.Msg)
	for i, tc := range tests {
		inaddr, _ := dns.ReverseAddr(tc.addr)
		m.SetQuestion(inaddr, dns.TypeA)

		r, e := dns.Exchange(m, udp)
		if e != nil {
			t.Errorf("Test %d, expected no error, got %q", i, e)
		}
		if r.Rcode != tc.rcode {
			t.Errorf("Test %d, expected %d, got %d for %s", i, tc.rcode, r.Rcode, tc.addr)
		}
	}
}

func TestReverse(t *testing.T) {
	corefile := `192.168.1.0/24:0 {
		whoami
}
`
	s, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer s.Stop()

	tests := []struct {
		addr  string
		rcode int
	}{
		{"192.168.1.0", dns.RcodeSuccess},
		{"192.168.1.1", dns.RcodeSuccess},
		{"192.168.1.127", dns.RcodeSuccess},
		{"192.168.1.128", dns.RcodeSuccess},
		{"1.168.192.in-addr.arpa.", dns.RcodeSuccess},

		{"2.168.192.in-addr.arpa.", dns.RcodeRefused},
	}

	m := new(dns.Msg)
	for i, tc := range tests {
		inaddr := tc.addr
		var err error
		if !strings.HasSuffix(tc.addr, ".arpa.") {
			inaddr, err = dns.ReverseAddr(tc.addr)
			if err != nil {
				t.Fatalf("Test %d, failed to convert %s", i, tc.addr)
			}
			tc.addr = inaddr
		}

		m.SetQuestion(tc.addr, dns.TypeA)

		r, e := dns.Exchange(m, udp)
		if e != nil {
			t.Errorf("Test %d, expected no error, got %q", i, e)
		}
		if r.Rcode != tc.rcode {
			t.Errorf("Test %d, expected %d, got %d for %s", i, tc.rcode, r.Rcode, tc.addr)
		}
	}
}

func TestReverseInAddr(t *testing.T) {
	corefile := `1.168.192.in-addr.arpa:0 {
		whoami
}
`
	s, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer s.Stop()

	tests := []struct {
		addr  string
		rcode int
	}{
		{"192.168.1.0", dns.RcodeSuccess},
		{"192.168.1.1", dns.RcodeSuccess},
		{"192.168.1.127", dns.RcodeSuccess},
		{"192.168.1.128", dns.RcodeSuccess},
		{"1.168.192.in-addr.arpa.", dns.RcodeSuccess},

		{"2.168.192.in-addr.arpa.", dns.RcodeRefused},
	}

	m := new(dns.Msg)
	for i, tc := range tests {
		inaddr := tc.addr
		var err error
		if !strings.HasSuffix(tc.addr, ".arpa.") {
			inaddr, err = dns.ReverseAddr(tc.addr)
			if err != nil {
				t.Fatalf("Test %d, failed to convert %s", i, tc.addr)
			}
			tc.addr = inaddr
		}

		m.SetQuestion(tc.addr, dns.TypeA)

		r, e := dns.Exchange(m, udp)
		if e != nil {
			t.Errorf("Test %d, expected no error, got %q", i, e)
		}
		if r.Rcode != tc.rcode {
			t.Errorf("Test %d, expected %d, got %d for %s", i, tc.rcode, r.Rcode, tc.addr)
		}
	}
}
