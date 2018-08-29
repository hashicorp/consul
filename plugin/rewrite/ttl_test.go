package rewrite

import (
	"context"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestNewTtlRule(t *testing.T) {
	tests := []struct {
		next         string
		args         []string
		expectedFail bool
	}{
		{"stop", []string{"srv1.coredns.rocks", "10"}, false},
		{"stop", []string{"exact", "srv1.coredns.rocks", "15"}, false},
		{"stop", []string{"prefix", "coredns.rocks", "20"}, false},
		{"stop", []string{"suffix", "srv1", "25"}, false},
		{"stop", []string{"substring", "coredns", "30"}, false},
		{"stop", []string{"regex", `(srv1)\.(coredns)\.(rocks)`, "35"}, false},
		{"continue", []string{"srv1.coredns.rocks", "10"}, false},
		{"continue", []string{"exact", "srv1.coredns.rocks", "15"}, false},
		{"continue", []string{"prefix", "coredns.rocks", "20"}, false},
		{"continue", []string{"suffix", "srv1", "25"}, false},
		{"continue", []string{"substring", "coredns", "30"}, false},
		{"continue", []string{"regex", `(srv1)\.(coredns)\.(rocks)`, "35"}, false},
		{"stop", []string{"srv1.coredns.rocks", "12345678901234567890"}, true},
		{"stop", []string{"srv1.coredns.rocks", "coredns.rocks"}, true},
		{"stop", []string{"srv1.coredns.rocks", "-1"}, true},
	}
	for i, tc := range tests {
		failed := false
		rule, err := newTtlRule(tc.next, tc.args...)
		if err != nil {
			failed = true
		}
		if !failed && !tc.expectedFail {
			t.Logf("Test %d: PASS, passed as expected: (%s) %s", i, tc.next, tc.args)
			continue
		}
		if failed && tc.expectedFail {
			t.Logf("Test %d: PASS, failed as expected: (%s) %s: %s", i, tc.next, tc.args, err)
			continue
		}
		t.Fatalf("Test %d: FAIL, expected fail=%t, but received fail=%t: (%s) %s, rule=%v", i, tc.expectedFail, failed, tc.next, tc.args, rule)
	}
	for i, tc := range tests {
		failed := false
		tc.args = append([]string{tc.next, "ttl"}, tc.args...)
		rule, err := newRule(tc.args...)
		if err != nil {
			failed = true
		}
		if !failed && !tc.expectedFail {
			t.Logf("Test %d: PASS, passed as expected: (%s) %s", i, tc.next, tc.args)
			continue
		}
		if failed && tc.expectedFail {
			t.Logf("Test %d: PASS, failed as expected: (%s) %s: %s", i, tc.next, tc.args, err)
			continue
		}
		t.Fatalf("Test %d: FAIL, expected fail=%t, but received fail=%t: (%s) %s, rule=%v", i, tc.expectedFail, failed, tc.next, tc.args, rule)
	}
}

func TestTtlRewrite(t *testing.T) {
	rules := []Rule{}
	ruleset := []struct {
		args         []string
		expectedType reflect.Type
	}{
		{[]string{"stop", "ttl", "srv1.coredns.rocks", "1"}, reflect.TypeOf(&exactTtlRule{})},
		{[]string{"stop", "ttl", "exact", "srv15.coredns.rocks", "15"}, reflect.TypeOf(&exactTtlRule{})},
		{[]string{"stop", "ttl", "prefix", "srv30", "30"}, reflect.TypeOf(&prefixTtlRule{})},
		{[]string{"stop", "ttl", "suffix", "45.coredns.rocks", "45"}, reflect.TypeOf(&suffixTtlRule{})},
		{[]string{"stop", "ttl", "substring", "rv50", "50"}, reflect.TypeOf(&substringTtlRule{})},
		{[]string{"stop", "ttl", "regex", `(srv10)\.(coredns)\.(rocks)`, "10"}, reflect.TypeOf(&regexTtlRule{})},
		{[]string{"stop", "ttl", "regex", `(srv20)\.(coredns)\.(rocks)`, "20"}, reflect.TypeOf(&regexTtlRule{})},
	}
	for i, r := range ruleset {
		rule, err := newRule(r.args...)
		if err != nil {
			t.Fatalf("Rule %d: FAIL, %s: %s", i, r.args, err)
			continue
		}
		if reflect.TypeOf(rule) != r.expectedType {
			t.Fatalf("Rule %d: FAIL, %s: rule type mismatch, expected %q, but got %q", i, r.args, r.expectedType, rule)
		}
		rules = append(rules, rule)
	}
	doTtlTests(rules, t)
}

func doTtlTests(rules []Rule, t *testing.T) {
	tests := []struct {
		from      string
		fromType  uint16
		answer    []dns.RR
		ttl       uint32
		noRewrite bool
	}{
		{"srv1.coredns.rocks.", dns.TypeA, []dns.RR{test.A("srv1.coredns.rocks.  5   IN  A  10.0.0.1")}, 1, false},
		{"srv15.coredns.rocks.", dns.TypeA, []dns.RR{test.A("srv15.coredns.rocks.  5   IN  A  10.0.0.15")}, 15, false},
		{"srv30.coredns.rocks.", dns.TypeA, []dns.RR{test.A("srv30.coredns.rocks.  5   IN  A  10.0.0.30")}, 30, false},
		{"srv45.coredns.rocks.", dns.TypeA, []dns.RR{test.A("srv45.coredns.rocks.  5   IN  A  10.0.0.45")}, 45, false},
		{"srv50.coredns.rocks.", dns.TypeA, []dns.RR{test.A("srv50.coredns.rocks.  5   IN  A  10.0.0.50")}, 50, false},
		{"srv10.coredns.rocks.", dns.TypeA, []dns.RR{test.A("srv10.coredns.rocks.  5   IN  A  10.0.0.10")}, 10, false},
		{"xmpp.coredns.rocks.", dns.TypeSRV, []dns.RR{test.SRV("xmpp.coredns.rocks.  5  IN  SRV 0 100 100 srvxmpp.coredns.rocks.")}, 5, true},
		{"srv15.coredns.rocks.", dns.TypeHINFO, []dns.RR{test.HINFO("srv15.coredns.rocks.  5  HINFO INTEL-64 \"RHEL 7.5\"")}, 15, false},
		{"srv20.coredns.rocks.", dns.TypeA, []dns.RR{
			test.A("srv20.coredns.rocks.  5   IN  A  10.0.0.22"),
			test.A("srv20.coredns.rocks.  5   IN  A  10.0.0.23"),
		}, 20, false},
	}
	ctx := context.TODO()
	for i, tc := range tests {
		failed := false
		m := new(dns.Msg)
		m.SetQuestion(tc.from, tc.fromType)
		m.Question[0].Qclass = dns.ClassINET
		m.Answer = tc.answer
		rw := Rewrite{
			Next:     plugin.HandlerFunc(msgPrinter),
			Rules:    rules,
			noRevert: false,
		}
		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		rw.ServeDNS(ctx, rec, m)
		resp := rec.Msg
		if len(resp.Answer) == 0 {
			t.Errorf("Test %d: FAIL %s (%d) Expected valid response but received %q", i, tc.from, tc.fromType, resp)
			failed = true
			continue
		}
		for _, a := range resp.Answer {
			if a.Header().Ttl != tc.ttl {
				t.Errorf("Test %d: FAIL %s (%d) Expected TTL to be %d but was %d", i, tc.from, tc.fromType, tc.ttl, a.Header().Ttl)
				failed = true
				break
			}
		}
		if !failed {
			if tc.noRewrite {
				t.Logf("Test %d: PASS %s (%d) worked as expected, no rewrite for ttl %d", i, tc.from, tc.fromType, tc.ttl)
			} else {
				t.Logf("Test %d: PASS %s (%d) worked as expected, rewrote ttl to %d", i, tc.from, tc.fromType, tc.ttl)
			}
		}
	}
}
