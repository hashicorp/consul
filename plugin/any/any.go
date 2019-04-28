package any

import (
	"context"

	"github.com/coredns/coredns/plugin"

	"github.com/miekg/dns"
)

// Any is a plugin that returns a HINFO reply to ANY queries.
type Any struct {
	Next plugin.Handler
}

// ServeDNS implements the plugin.Handler interface.
func (a Any) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	if r.Question[0].Qtype != dns.TypeANY {
		return plugin.NextOrFailure(a.Name(), a.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	hdr := dns.RR_Header{Name: r.Question[0].Name, Ttl: 8482, Class: dns.ClassINET, Rrtype: dns.TypeHINFO}
	m.Answer = []dns.RR{&dns.HINFO{Hdr: hdr, Cpu: "ANY obsoleted", Os: "See RFC 8482"}}

	w.WriteMsg(m)
	return 0, nil
}

// Name implements the Handler interface.
func (a Any) Name() string { return "any" }
