// Package loadbalance shuffles A and AAAA records.
package loadbalance

import (
	"log"

	"github.com/miekg/dns"
)

// RoundRobinResponseWriter is a response writer that shuffles A and AAAA records.
type RoundRobinResponseWriter struct {
	dns.ResponseWriter
}

// WriteMsg implements the dns.ResponseWriter interface.
func (r *RoundRobinResponseWriter) WriteMsg(res *dns.Msg) error {
	if res.Rcode != dns.RcodeSuccess {
		return r.ResponseWriter.WriteMsg(res)
	}

	res.Answer = roundRobin(res.Answer)
	res.Ns = roundRobin(res.Ns)
	res.Extra = roundRobin(res.Extra)

	return r.ResponseWriter.WriteMsg(res)
}

func roundRobin(in []dns.RR) []dns.RR {
	cname := []dns.RR{}
	address := []dns.RR{}
	rest := []dns.RR{}
	for _, r := range in {
		switch r.Header().Rrtype {
		case dns.TypeCNAME:
			cname = append(cname, r)
		case dns.TypeA, dns.TypeAAAA:
			address = append(address, r)
		default:
			rest = append(rest, r)
		}
	}

	switch l := len(address); l {
	case 0, 1:
		break
	case 2:
		if dns.Id()%2 == 0 {
			address[0], address[1] = address[1], address[0]
		}
	default:
		for j := 0; j < l*(int(dns.Id())%4+1); j++ {
			q := int(dns.Id()) % l
			p := int(dns.Id()) % l
			if q == p {
				p = (p + 1) % l
			}
			address[q], address[p] = address[p], address[q]
		}
	}
	out := append(cname, rest...)
	out = append(out, address...)
	return out
}

// Write implements the dns.ResponseWriter interface.
func (r *RoundRobinResponseWriter) Write(buf []byte) (int, error) {
	// Should we pack and unpack here to fiddle with the packet... Not likely.
	log.Printf("[WARNING] RoundRobin called with Write: no shuffling records")
	n, err := r.ResponseWriter.Write(buf)
	return n, err
}

// Hijack implements the dns.ResponseWriter interface.
func (r *RoundRobinResponseWriter) Hijack() {
	r.ResponseWriter.Hijack()
	return
}
