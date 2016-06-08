package etcd

import (
	"fmt"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (e Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}
	if state.QClass() != dns.ClassINET {
		return dns.RcodeServerFailure, fmt.Errorf("can only deal with ClassINET")
	}
	name := state.Name()
	if e.Debug {
		if debug := isDebug(name); debug != "" {
			e.debug = r.Question[0].Name
			state.Clear()
			state.Req.Question[0].Name = debug
		}
	}

	// We need to check stubzones first, because we may get a request for a zone we
	// are not auth. for *but* do have a stubzone forward for. If we do the stubzone
	// handler will handle the request.
	if e.Stubmap != nil && len(*e.Stubmap) > 0 {
		for zone, _ := range *e.Stubmap {
			if middleware.Name(zone).Matches(name) {
				stub := Stub{Etcd: e, Zone: zone}
				return stub.ServeDNS(ctx, w, r)
			}
		}
	}

	zone := middleware.Zones(e.Zones).Matches(state.Name())
	if zone == "" {
		if e.Next == nil {
			return dns.RcodeServerFailure, nil
		}
		return e.Next.ServeDNS(ctx, w, r)
	}

	var (
		records, extra []dns.RR
		debug          []msg.Service
		err            error
	)
	switch state.Type() {
	case "A":
		records, debug, err = e.A(zone, state, nil)
	case "AAAA":
		records, debug, err = e.AAAA(zone, state, nil)
	case "TXT":
		records, debug, err = e.TXT(zone, state)
	case "CNAME":
		records, debug, err = e.CNAME(zone, state)
	case "PTR":
		records, debug, err = e.PTR(zone, state)
	case "MX":
		records, extra, debug, err = e.MX(zone, state)
	case "SRV":
		records, extra, debug, err = e.SRV(zone, state)
	case "SOA":
		records, debug, err = e.SOA(zone, state)
	case "NS":
		if state.Name() == zone {
			records, extra, debug, err = e.NS(zone, state)
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, debug, err = e.A(zone, state, nil)
	}

	if e.debug != "" {
		// Substitute this name with the original when we return the request.
		state.Clear()
		state.Req.Question[0].Name = e.debug
	}

	if isEtcdNameError(err) {
		return e.Err(zone, dns.RcodeNameError, state, debug, err)
	}
	if err != nil {
		return e.Err(zone, dns.RcodeServerFailure, state, debug, err)
	}

	if len(records) == 0 {
		return e.Err(zone, dns.RcodeSuccess, state, debug, err)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)
	if e.debug != "" {
		m.Extra = append(m.Extra, servicesToTxt(debug)...)
	}

	m = dedup(m)
	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Err write an error response to the client.
func (e Etcd) Err(zone string, rcode int, state middleware.State, debug []msg.Service, err error) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Ns, _, _ = e.SOA(zone, state)
	if e.debug != "" {
		m.Extra = servicesToTxt(debug)
		txt := errorToTxt(err)
		if txt != nil {
			m.Extra = append(m.Extra, errorToTxt(err))
		}
	}
	state.SizeAndDo(m)
	state.W.WriteMsg(m)
	// Return success as the rcode to signal we have written to the client.
	return dns.RcodeSuccess, nil
}

func dedup(m *dns.Msg) *dns.Msg {
	// TODO(miek): expensive!
	m.Answer = dns.Dedup(m.Answer, nil)
	m.Ns = dns.Dedup(m.Ns, nil)
	m.Extra = dns.Dedup(m.Extra, nil)
	return m
}
