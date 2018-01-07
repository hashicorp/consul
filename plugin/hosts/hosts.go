package hosts

import (
	"net"

	"golang.org/x/net/context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

// Hosts is the plugin handler
type Hosts struct {
	Next plugin.Handler
	*Hostsfile

	Fall fall.F
}

// ServeDNS implements the plugin.Handle interface.
func (h Hosts) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	answers := []dns.RR{}

	zone := plugin.Zones(h.Origins).Matches(qname)
	if zone == "" {
		// PTR zones don't need to be specified in Origins
		if state.Type() != "PTR" {
			// If this doesn't match we need to fall through regardless of h.Fallthrough
			return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
		}
	}

	switch state.QType() {
	case dns.TypePTR:
		names := h.LookupStaticAddr(dnsutil.ExtractAddressFromReverse(qname))
		if len(names) == 0 {
			// If this doesn't match we need to fall through regardless of h.Fallthrough
			return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
		}
		answers = h.ptr(qname, names)
	case dns.TypeA:
		ips := h.LookupStaticHostV4(qname)
		answers = a(qname, ips)
	case dns.TypeAAAA:
		ips := h.LookupStaticHostV6(qname)
		answers = aaaa(qname, ips)
	}

	if len(answers) == 0 {
		if h.Fall.Through(qname) {
			return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
		}
		if !h.otherRecordsExist(state.QType(), qname) {
			return dns.RcodeNameError, nil
		}
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Answer = answers

	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

func (h Hosts) otherRecordsExist(qtype uint16, qname string) bool {
	switch qtype {
	case dns.TypeA:
		if len(h.LookupStaticHostV6(qname)) > 0 {
			return true
		}
	case dns.TypeAAAA:
		if len(h.LookupStaticHostV4(qname)) > 0 {
			return true
		}
	default:
		if len(h.LookupStaticHostV4(qname)) > 0 {
			return true
		}
		if len(h.LookupStaticHostV6(qname)) > 0 {
			return true
		}
	}
	return false

}

// Name implements the plugin.Handle interface.
func (h Hosts) Name() string { return "hosts" }

// a takes a slice of net.IPs and returns a slice of A RRs.
func a(zone string, ips []net.IP) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.A)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeA,
			Class: dns.ClassINET, Ttl: 3600}
		r.A = ip
		answers = append(answers, r)
	}
	return answers
}

// aaaa takes a slice of net.IPs and returns a slice of AAAA RRs.
func aaaa(zone string, ips []net.IP) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.AAAA)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeAAAA,
			Class: dns.ClassINET, Ttl: 3600}
		r.AAAA = ip
		answers = append(answers, r)
	}
	return answers
}

// ptr takes a slice of host names and filters out the ones that aren't in Origins, if specified, and returns a slice of PTR RRs.
func (h *Hosts) ptr(zone string, names []string) []dns.RR {
	answers := []dns.RR{}
	for _, n := range names {
		r := new(dns.PTR)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypePTR,
			Class: dns.ClassINET, Ttl: 3600}
		r.Ptr = dns.Fqdn(n)
		answers = append(answers, r)
	}
	return answers
}
