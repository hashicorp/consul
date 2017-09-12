package etcd

import (
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (e *Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	opt := middleware.Options{}
	state := request.Request{W: w, Req: r}

	name := state.Name()

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
		return middleware.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	var (
		records, extra []dns.RR
		err            error
	)
	switch state.Type() {
	case "A":
		records, err = middleware.A(e, zone, state, nil, opt)
	case "AAAA":
		records, err = middleware.AAAA(e, zone, state, nil, opt)
	case "TXT":
		records, err = middleware.TXT(e, zone, state, opt)
	case "CNAME":
		records, err = middleware.CNAME(e, zone, state, opt)
	case "PTR":
		records, err = middleware.PTR(e, zone, state, opt)
	case "MX":
		records, extra, err = middleware.MX(e, zone, state, opt)
	case "SRV":
		records, extra, err = middleware.SRV(e, zone, state, opt)
	case "SOA":
		records, err = middleware.SOA(e, zone, state, opt)
	case "NS":
		if state.Name() == zone {
			records, extra, err = middleware.NS(e, zone, state, opt)
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, err = middleware.A(e, zone, state, nil, opt)
	}

	if e.IsNameError(err) {
		if e.Fallthrough {
			return middleware.NextOrFailure(e.Name(), e.Next, ctx, w, r)
		}
		// Make err nil when returning here, so we don't log spam for NXDOMAIN.
		return middleware.BackendError(e, zone, dns.RcodeNameError, state, nil /* err */, opt)
	}
	if err != nil {
		return middleware.BackendError(e, zone, dns.RcodeServerFailure, state, err, opt)
	}

	if len(records) == 0 {
		return middleware.BackendError(e, zone, dns.RcodeSuccess, state, err, opt)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)

	m = dnsutil.Dedup(m)
	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (e *Etcd) Name() string { return "etcd" }
