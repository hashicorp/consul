package reverse

import (
	"net"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Reverse provides dynamic reverse DNS and the related forward RR.
type Reverse struct {
	Next        middleware.Handler
	Networks    networks
	Fallthrough bool
}

// ServeDNS implements the middleware.Handler interface.
func (re Reverse) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	var rr dns.RR

	state := request.Request{W: w, Req: r}
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	switch state.QType() {
	case dns.TypePTR:
		address := dnsutil.ExtractAddressFromReverse(state.Name())

		if address == "" {
			// Not an reverse lookup, but can still be an pointer for an domain
			break
		}

		ip := net.ParseIP(address)
		// loop through the configured networks
		for _, n := range re.Networks {
			if n.IPnet.Contains(ip) {
				rr = &dns.PTR{
					Hdr: dns.RR_Header{Name: state.QName(), Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: n.TTL},
					Ptr: n.ipToHostname(ip),
				}
				break
			}
		}

	case dns.TypeA:
		for _, n := range re.Networks {
			if dns.IsSubDomain(n.Zone, state.Name()) {

				// skip if requesting an v4 address and network is not v4
				if n.IPnet.IP.To4() == nil {
					continue
				}

				result := n.hostnameToIP(state.Name())
				if result != nil {
					rr = &dns.A{
						Hdr: dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: n.TTL},
						A:   result,
					}
					break
				}
			}
		}

	case dns.TypeAAAA:
		for _, n := range re.Networks {
			if dns.IsSubDomain(n.Zone, state.Name()) {

				// Do not use To16 which tries to make v4 in v6
				if n.IPnet.IP.To4() != nil {
					continue
				}

				result := n.hostnameToIP(state.Name())
				if result != nil {
					rr = &dns.AAAA{
						Hdr:  dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: n.TTL},
						AAAA: result,
					}
					break
				}
			}
		}

	}

	if rr != nil {
		m.Answer = append(m.Answer, rr)
		state.SizeAndDo(m)
		w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	}

	if re.Fallthrough {
		return middleware.NextOrFailure(re.Name(), re.Next, ctx, w, r)
	}
	return dns.RcodeServerFailure, nil
}

// Name implements the Handler interface.
func (re Reverse) Name() string { return "reverse" }
