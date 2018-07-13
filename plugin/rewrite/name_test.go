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
