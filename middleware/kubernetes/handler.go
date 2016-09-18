package kubernetes

import (
	"fmt"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (k Kubernetes) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	if state.QClass() != dns.ClassINET {
		return dns.RcodeServerFailure, fmt.Errorf("can only deal with ClassINET")
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	// TODO: find an alternative to this block
	ip := dnsutil.ExtractAddressFromReverse(state.Name())
	if ip != "" {
		records := k.getServiceRecordForIP(ip, state.Name())
		if len(records) > 0 {
			srvPTR := &records[0]
			m.Answer = append(m.Answer, srvPTR.NewPTR(state.QName(), ip))

			m = dedup(m)
			state.SizeAndDo(m)
			m, _ = state.Scrub(m)
			w.WriteMsg(m)
			return dns.RcodeSuccess, nil
		}
	}

	// Check that query matches one of the zones served by this middleware,
	// otherwise delegate to the next in the pipeline.
	zone := middleware.Zones(k.Zones).Matches(state.Name())
	if zone == "" {
		if k.Next == nil {
			return dns.RcodeServerFailure, nil
		}
		return k.Next.ServeDNS(ctx, w, r)
	}

	var (
		records, extra []dns.RR
		err            error
	)
	switch state.Type() {
	case "A":
		records, err = k.A(zone, state, nil)
	case "AAAA":
		records, err = k.AAAA(zone, state, nil)
	case "TXT":
		records, err = k.TXT(zone, state)
		// TODO: change lookup to return appropriate error. Then add code below
		// this switch to check for the error and return not implemented.
		//return dns.RcodeNotImplemented, nil
	case "CNAME":
		records, err = k.CNAME(zone, state)
	case "MX":
		records, extra, err = k.MX(zone, state)
	case "SRV":
		records, extra, err = k.SRV(zone, state)
	case "SOA":
		records = []dns.RR{k.SOA(zone, state)}
	case "NS":
		if state.Name() == zone {
			records, extra, err = k.NS(zone, state)
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, err = k.A(zone, state, nil)
	}
	if isKubernetesNameError(err) {
		return k.Err(zone, dns.RcodeNameError, state)
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if len(records) == 0 {
		return k.Err(zone, dns.RcodeSuccess, state)
	}

	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)

	m = dedup(m)
	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// NoData write a nodata response to the client.
func (k Kubernetes) Err(zone string, rcode int, state request.Request) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Ns = []dns.RR{k.SOA(zone, state)}
	state.SizeAndDo(m)
	state.W.WriteMsg(m)
	return rcode, nil
}

func dedup(m *dns.Msg) *dns.Msg {
	// TODO(miek): expensive!
	m.Answer = dns.Dedup(m.Answer, nil)
	m.Ns = dns.Dedup(m.Ns, nil)
	m.Extra = dns.Dedup(m.Extra, nil)
	return m
}
