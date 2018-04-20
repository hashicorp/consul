package rewrite

import (
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"context"

	"github.com/miekg/dns"
)

func TestResponseReverter(t *testing.T) {
	rules := []Rule{}
	r, _ := newNameRule("stop", "regex", `(core)\.(dns)\.(rocks)`, "{2}.{1}.{3}", "answer", "name", `(dns)\.(core)\.(rocks)`, "{2}.{1}.{3}")
	rules = append(rules, r)

	tests := []struct {
		from     string
		fromType uint16
		answer   []dns.RR
		to       string
		toType   uint16
		noRevert bool
	}{
		{"core.dns.rocks", dns.TypeA, []dns.RR{test.A("dns.core.rocks.  5   IN  A  10.0.0.1")}, "core.dns.rocks", dns.TypeA, false},
		{"core.dns.rocks", dns.TypeSRV, []dns.RR{test.SRV("dns.core.rocks.  5  IN  SRV 0 100 100 srv1.dns.core.rocks.")}, "core.dns.rocks", dns.TypeSRV, false},
		{"core.dns.rocks", dns.TypeA, []dns.RR{test.A("core.dns.rocks.  5   IN  A  10.0.0.1")}, "dns.core.rocks.", dns.TypeA, true},
		{"core.dns.rocks", dns.TypeSRV, []dns.RR{test.SRV("core.dns.rocks.  5  IN  SRV 0 100 100 srv1.dns.core.rocks.")}, "dns.core.rocks.", dns.TypeSRV, true},
		{"core.dns.rocks", dns.TypeHINFO, []dns.RR{test.HINFO("core.dns.rocks.  5  HINFO INTEL-64 \"RHEL 7.4\"")}, "core.dns.rocks", dns.TypeHINFO, false},
		{"core.dns.rocks", dns.TypeA, []dns.RR{
			test.A("dns.core.rocks.  5   IN  A  10.0.0.1"),
			test.A("dns.core.rocks.  5   IN  A  10.0.0.2"),
		}, "core.dns.rocks", dns.TypeA, false},
	}

	ctx := context.TODO()
	for i, tc := range tests {
		m := new(dns.Msg)
		m.SetQuestion(tc.from, tc.fromType)
		m.Question[0].Qclass = dns.ClassINET
		m.Answer = tc.answer
		rw := Rewrite{
			Next:     plugin.HandlerFunc(msgPrinter),
			Rules:    rules,
			noRevert: tc.noRevert,
		}
		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		rw.ServeDNS(ctx, rec, m)
		resp := rec.Msg
		if resp.Question[0].Name != tc.to {
			t.Errorf("Test %d: Expected Name to be %q but was %q", i, tc.to, resp.Question[0].Name)
		}
		if resp.Question[0].Qtype != tc.toType {
			t.Errorf("Test %d: Expected Type to be '%d' but was '%d'", i, tc.toType, resp.Question[0].Qtype)
		}
	}
}
