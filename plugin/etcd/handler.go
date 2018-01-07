package etcd

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the plugin.Handler interface.
func (e *Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	opt := plugin.Options{}
	state := request.Request{W: w, Req: r}

	name := state.Name()

	// We need to check stubzones first, because we may get a request for a zone we
	// are not auth. for *but* do have a stubzone forward for. If we do the stubzone
	// handler will handle the request.
	if e.Stubmap != nil && len(*e.Stubmap) > 0 {
		for zone := range *e.Stubmap {
			if plugin.Name(zone).Matches(name) {
				stub := Stub{Etcd: e, Zone: zone}
				return stub.ServeDNS(ctx, w, r)
			}
		}
	}

	zone := plugin.Zones(e.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	var (
		records, extra []dns.RR
		err            error
	)

	switch state.QType() {
	case dns.TypeA:
		records, err = plugin.A(e, zone, state, nil, opt)
	case dns.TypeAAAA:
		records, err = plugin.AAAA(e, zone, state, nil, opt)
	case dns.TypeTXT:
		records, err = plugin.TXT(e, zone, state, opt)
	case dns.TypeCNAME:
		records, err = plugin.CNAME(e, zone, state, opt)
	case dns.TypePTR:
		records, err = plugin.PTR(e, zone, state, opt)
	case dns.TypeMX:
		records, extra, err = plugin.MX(e, zone, state, opt)
	case dns.TypeSRV:
		records, extra, err = plugin.SRV(e, zone, state, opt)
	case dns.TypeSOA:
		records, err = plugin.SOA(e, zone, state, opt)
	case dns.TypeNS:
		if state.Name() == zone {
			records, extra, err = plugin.NS(e, zone, state, opt)
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, err = plugin.A(e, zone, state, nil, opt)
	}

	if e.IsNameError(err) {
		if e.Fall.Through(state.Name()) {
			return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
		}
		// Make err nil when returning here, so we don't log spam for NXDOMAIN.
		return plugin.BackendError(e, zone, dns.RcodeNameError, state, nil /* err */, opt)
	}
	if err != nil {
		return plugin.BackendError(e, zone, dns.RcodeServerFailure, state, err, opt)
	}

	if len(records) == 0 {
		return plugin.BackendError(e, zone, dns.RcodeSuccess, state, err, opt)
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
