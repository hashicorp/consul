package rewrite

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestRewriteIllegalName(t *testing.T) {
	r, _ := newNameRule("stop", "example.org.", "example..org.")

	rw := Rewrite{
		Next:     plugin.HandlerFunc(msgPrinter),
		Rules:    []Rule{r},
		noRevert: true,
	}

	ctx := context.TODO()
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := rw.ServeDNS(ctx, rec, m)
	if !strings.Contains(err.Error(), "invalid name") {
		t.Errorf("Expected invalid name, got %s", err.Error())
	}
}

func TestRewriteNamePrefix(t *testing.T) {
	r, err := newNameRule("stop", "prefix", "test", "not-a-test")
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}

	rw := Rewrite{
		Next:     plugin.HandlerFunc(msgPrinter),
		Rules:    []Rule{r},
		noRevert: true,
	}

	ctx := context.TODO()
	m := new(dns.Msg)
	m.SetQuestion("test.example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err = rw.ServeDNS(ctx, rec, m)
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	expected := "not-a-test.example.org."
	actual := rec.Msg.Question[0].Name
	if actual != expected {
		t.Fatalf("Expected rewrite to %v, got %v", expected, actual)
	}
}

func TestNewNameRule(t *testing.T) {
	tests := []struct {
		next         string
		args         []string
		expectedFail bool
	}{
		{"stop", []string{"exact", "srv3.coredns.rocks", "srv4.coredns.rocks"}, false},
		{"stop", []string{"srv1.coredns.rocks", "srv2.coredns.rocks"}, false},
		{"stop", []string{"suffix", "coredns.rocks", "coredns.rocks."}, false},
		{"stop", []string{"suffix", "coredns.rocks.", "coredns.rocks"}, false},
		{"stop", []string{"suffix", "coredns.rocks.", "coredns.rocks."}, false},
		{"stop", []string{"regex", "srv1.coredns.rocks", "10"}, false},
		{"stop", []string{"regex", "(.*).coredns.rocks", "10"}, false},
		{"stop", []string{"regex", "(.*).coredns.rocks", "{1}.coredns.rocks"}, false},
		{"stop", []string{"regex", "(.*).coredns.rocks", "{1}.{2}.coredns.rocks"}, true},
		{"stop", []string{"regex", "staging.mydomain.com", "aws-loadbalancer-id.us-east-1.elb.amazonaws.com"}, false},
	}
	for i, tc := range tests {
		failed := false
		rule, err := newNameRule(tc.next, tc.args...)
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
		if failed && !tc.expectedFail {
			t.Fatalf("Test %d: FAIL, expected fail=%t, but received fail=%t: (%s) %s, rule=%v, error=%s", i, tc.expectedFail, failed, tc.next, tc.args, rule, err)
		}
		t.Fatalf("Test %d: FAIL, expected fail=%t, but received fail=%t: (%s) %s, rule=%v", i, tc.expectedFail, failed, tc.next, tc.args, rule)
	}
	for i, tc := range tests {
		failed := false
		tc.args = append([]string{tc.next, "name"}, tc.args...)
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
