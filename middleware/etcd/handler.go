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
	opt := middleware.Options{}
	state := request.Request{W: w, Req: r}
	if state.QClass() != dns.ClassINET {
		return dns.RcodeServerFailure, fmt.Errorf("can only deal with ClassINET")
	}
	name := state.Name()
	if e.Debugging {
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
		records, debug, err = middleware.A(e, zone, state, nil, opt)
	case "AAAA":
		records, debug, err = middleware.AAAA(e, zone, state, nil, opt)
	case "TXT":
		records, debug, err = middleware.TXT(e, zone, state, opt)
	case "CNAME":
		records, debug, err = middleware.CNAME(e, zone, state, opt)
	case "PTR":
		records, debug, err = middleware.PTR(e, zone, state, opt)
	case "MX":
		records, extra, debug, err = middleware.MX(e, zone, state, opt)
	case "SRV":
		records, extra, debug, err = middleware.SRV(e, zone, state, opt)
	case "SOA":
		records, debug, err = middleware.SOA(e, zone, state, opt)
	case "NS":
		if state.Name() == zone {
			records, extra, debug, err = middleware.NS(e, zone, state, opt)
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, debug, err = middleware.A(e, zone, state, nil, opt)
	}

	if opt.Debug != "" {
		// Substitute this name with the original when we return the request.
		state.Clear()
		state.Req.Question[0].Name = opt.Debug
	}

	if e.IsNameError(err) {
		return middleware.BackendError(e, zone, dns.RcodeNameError, state, debug, err, opt)
	}
	if err != nil {
		return middleware.BackendError(e, zone, dns.RcodeServerFailure, state, debug, err, opt)
	}

	if len(records) == 0 {
		return middleware.BackendError(e, zone, dns.RcodeSuccess, state, debug, err, opt)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)
	if opt.Debug != "" {
		m.Extra = append(m.Extra, middleware.ServicesToTxt(debug)...)
	}

	m = dnsutil.Dedup(m)
	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (e *Etcd) Name() string { return "etcd" }
