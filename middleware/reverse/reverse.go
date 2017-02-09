package reverse

import (
	"net"

	"github.com/miekg/coredns/request"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Reverse provides dynamic reverse dns and the related forward rr
type Reverse struct {
	Next     middleware.Handler
	Networks networks
}

// ServeDNS implements the middleware.Handler interface.
func (reverse Reverse) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	var rr dns.RR
	nextHandler := true

	state := request.Request{W: w, Req: r}
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	switch state.QType(){
	case dns.TypePTR:
		address := dnsutil.ExtractAddressFromReverse(state.Name())

		if address == "" {
			// Not an reverse lookup, but can still be an pointer for an domain
			break
		}

		ip := net.ParseIP(address)
		// loop through the configured networks
		for _, n := range reverse.Networks {
			if (n.IPnet.Contains(ip)) {
				nextHandler = n.Fallthrough
				rr = &dns.PTR{
					Hdr:  dns.RR_Header{Name: state.QName(), Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: n.TTL},
					Ptr: n.ipToHostname(ip),
				}
				break
			}
		}

	case dns.TypeA:
		for _, n := range reverse.Networks {
			if dns.IsSubDomain(n.Zone, state.Name()) {
				nextHandler = n.Fallthrough

				// skip if requesting an v4 address and network is not v4
				if n.IPnet.IP.To4() == nil {
					continue
				}

				result := n.hostnameToIP(state.Name())
				if result != nil {
					rr = &dns.A{
						Hdr:  dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: n.TTL},
						A: result,
					}
					break
				}
			}
		}

	case dns.TypeAAAA:
		for _, n := range reverse.Networks {
			if dns.IsSubDomain(n.Zone, state.Name()) {
				nextHandler = n.Fallthrough

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

	if rr == nil {
		if reverse.Next == nil || !nextHandler {
			// could not resolv
			w.WriteMsg(m)
			return dns.RcodeNameError, nil
		}
		return reverse.Next.ServeDNS(ctx, w, r)
	}

	m.Answer = append(m.Answer, rr)
	state.SizeAndDo(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (reverse Reverse) Name() string {
	return "reverse"
}
