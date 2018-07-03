package cache

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestSpoof(t *testing.T) {
	// Send query for example.org, get reply for example.net; should not be cached.
	c := New()
	c.Next = spoofHandler(true)

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	c.ServeDNS(context.TODO(), rec, req)

	qname := rec.Msg.Question[0].Name
	if c.pcache.Len() != 0 {
		t.Errorf("Cached %s, while reply had %s", "example.org.", qname)
	}

	// qtype
	c.Next = spoofHandlerType()
	req.SetQuestion("example.org.", dns.TypeMX)

	c.ServeDNS(context.TODO(), rec, req)

	qtype := rec.Msg.Question[0].Qtype
	if c.pcache.Len() != 0 {
		t.Errorf("Cached %s type %d, while reply had %d", "example.org.", dns.TypeMX, qtype)
	}
}

func TestResponse(t *testing.T) {
	// Send query for example.org, get reply for example.net; should not be cached.
	c := New()
	c.Next = spoofHandler(false)

	req := new(dns.Msg)
	req.SetQuestion("example.net.", dns.TypeA)
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	c.ServeDNS(context.TODO(), rec, req)

	if c.pcache.Len() != 0 {
		t.Errorf("Cached %s, while reply had response set to %t", "example.net.", rec.Msg.Response)
	}
}

// spoofHandler is a fake plugin implementation which returns a single A records for example.org. The qname in the
// question section is set to example.NET (i.e. they *don't* match).
func spoofHandler(response bool) plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetQuestion("example.net.", dns.TypeA)
		m.Response = response
		m.Answer = []dns.RR{test.A("example.org. IN A 127.0.0.53")}
		w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	})
}

// spoofHandlerType is a fake plugin implementation which returns a single MX records for example.org. The qtype in the
// question section is set to A.
func spoofHandlerType() plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetQuestion("example.org.", dns.TypeA)
		m.Response = true
		m.Answer = []dns.RR{test.MX("example.org. IN MX 10 mail.example.org.")}
		w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	})
}
