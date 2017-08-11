package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

func TestParseRequest(t *testing.T) {
	k := Kubernetes{Zones: []string{zone}}

	tests := []struct {
		query    string
		qtype    uint16
		expected string // output from r.String()
	}{
		{
			// valid SRV request
			"_http._tcp.webs.mynamespace.svc.inter.webs.test.", dns.TypeSRV,
			"http.tcp..webs.mynamespace.svc.intern.webs.tests..",
		},
		{
			// wildcard acceptance
			"*.any.*.any.svc.inter.webs.test.", dns.TypeSRV,
			"*.any..*.any.svc.intern.webs.tests..",
		},
		{
			// A request of endpoint
			"1-2-3-4.webs.mynamespace.svc.inter.webs.test.", dns.TypeA,
			"..1-2-3-4.webs.mynamespace.svc.intern.webs.tests..",
		},
		{
			"inter.webs.test.", dns.TypeNS,
			"......intern.webs.tests..",
		},
	}
	for i, tc := range tests {
		m := new(dns.Msg)
		m.SetQuestion(tc.query, tc.qtype)
		state := request.Request{Zone: zone, Req: m}

		r, e := k.parseRequest(state)
		if e != nil {
			t.Errorf("Test %d, expected no error, got '%v'.", i, e)
		}
		rs := r.String()
		if rs != tc.expected {
			t.Errorf("Test %d, expected (stringyfied) recordRequest: %s, got %s", i, tc.expected, rs)
		}
	}
}

func TestParseInvalidRequest(t *testing.T) {
	k := Kubernetes{Zones: []string{zone}}

	invalid := map[string]uint16{
		"_http._tcp.webs.mynamespace.svc.inter.webs.test.": dns.TypeA,   // A requests cannot have port or protocol
		"_http._pcp.webs.mynamespace.svc.inter.webs.test.": dns.TypeSRV, // SRV protocol must be tcp or udp
		"_http._tcp.ep.webs.ns.svc.inter.webs.test.":       dns.TypeSRV, // SRV requests cannot have an endpoint
		"_*._*.webs.mynamespace.svc.inter.webs.test.":      dns.TypeSRV, // SRV request with invalid wildcards

	}

	for query, qtype := range invalid {
		m := new(dns.Msg)
		m.SetQuestion(query, qtype)
		state := request.Request{Zone: zone, Req: m}

		if _, e := k.parseRequest(state); e == nil {
			t.Errorf("Expected error from %s:%d, got none", query, qtype)
		}
	}
}

const zone = "intern.webs.tests."
