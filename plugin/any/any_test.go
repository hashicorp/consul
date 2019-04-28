package any

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestAny(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeANY)
	a := &Any{}

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := a.ServeDNS(context.TODO(), rec, req)

	if err != nil {
		t.Errorf("Expected no error, but got %q", err)
	}

	if rec.Msg.Answer[0].(*dns.HINFO).Cpu != "ANY obsoleted" {
		t.Errorf("Expected HINFO, but got %q", rec.Msg.Answer[0].(*dns.HINFO).Cpu)
	}
}
