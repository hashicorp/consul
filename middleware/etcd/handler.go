package etcd

import (
	"fmt"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (e *Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	opt := Options{}
	state := request.Request{W: w, Req: r}
	if state.QClass() != dns.ClassINET {
		return dns.RcodeServerFailure, fmt.Errorf("can only deal with ClassINET")
	}
	name := state.Name()
	if e.Debug {
		if debug := isDebug(name); debug != "" {
			opt.Debug = r.Question[0].Name
			state.Clear()
			state.Req.Question[0].Name = debug
		}
	}

	// We need to check stubzones first, because we may get a request for a zone we
	// are not auth. for *but* do have a stubzone forward for. If we do the stubzone
	// handler will handle the request.
	if e.Stubmap != nil && len(*e.Stubmap) > 0 {
		for zone := range *e.Stubmap {
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
		if opt.Debug != "" {
			r.Question[0].Name = opt.Debug
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
		records, debug, err = e.A(zone, state, nil, opt)
	case "AAAA":
		records, debug, err = e.AAAA(zone, state, nil, opt)
	case "TXT":
		records, debug, err = e.TXT(zone, state, opt)
	case "CNAME":
		records, debug, err = e.CNAME(zone, state, opt)
	case "PTR":
		records, debug, err = e.PTR(zone, state, opt)
	case "MX":
		records, extra, debug, err = e.MX(zone, state, opt)
	case "SRV":
		records, extra, debug, err = e.SRV(zone, state, opt)
	case "SOA":
		records, debug, err = e.SOA(zone, state, opt)
	case "NS":
		if state.Name() == zone {
			records, extra, debug, err = e.NS(zone, state, opt)
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, debug, err = e.A(zone, state, nil, opt)
	}

	if opt.Debug != "" {
		// Substitute this name with the original when we return the request.
		state.Clear()
		state.Req.Question[0].Name = opt.Debug
	}

	if isEtcdNameError(err) {
		return e.Err(zone, dns.RcodeNameError, state, debug, err, opt)
	}
	if err != nil {
		return e.Err(zone, dns.RcodeServerFailure, state, debug, err, opt)
	}

	if len(records) == 0 {
		return e.Err(zone, dns.RcodeSuccess, state, debug, err, opt)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)
	if opt.Debug != "" {
		m.Extra = append(m.Extra, servicesToTxt(debug)...)
	}

	m = dnsutil.Dedup(m)
	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

func (e *Etcd) Name() string { return "etcd" }

// Err write an error response to the client.
func (e *Etcd) Err(zone string, rcode int, state request.Request, debug []msg.Service, err error, opt Options) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Ns, _, _ = e.SOA(zone, state, opt)
	if opt.Debug != "" {
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
