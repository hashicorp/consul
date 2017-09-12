package kubernetes

import (
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (k Kubernetes) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	zone := middleware.Zones(k.Zones).Matches(state.Name())
	if zone == "" {
		return middleware.NextOrFailure(k.Name(), k.Next, ctx, w, r)
	}

	state.Zone = zone

	var (
		records []dns.RR
		extra   []dns.RR
		err     error
	)

	switch state.Type() {
	case "A":
		records, err = middleware.A(&k, zone, state, nil, middleware.Options{})
	case "AAAA":
		records, err = middleware.AAAA(&k, zone, state, nil, middleware.Options{})
	case "TXT":
		records, err = middleware.TXT(&k, zone, state, middleware.Options{})
	case "CNAME":
		records, err = middleware.CNAME(&k, zone, state, middleware.Options{})
	case "PTR":
		records, err = middleware.PTR(&k, zone, state, middleware.Options{})
	case "MX":
		records, extra, err = middleware.MX(&k, zone, state, middleware.Options{})
	case "SRV":
		records, extra, err = middleware.SRV(&k, zone, state, middleware.Options{})
	case "SOA":
		records, err = middleware.SOA(&k, zone, state, middleware.Options{})
	case "NS":
		if state.Name() == zone {
			records, extra, err = middleware.NS(&k, zone, state, middleware.Options{})
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, err = middleware.A(&k, zone, state, nil, middleware.Options{})
	}

	if k.IsNameError(err) {
		if k.Fallthrough {
			return middleware.NextOrFailure(k.Name(), k.Next, ctx, w, r)
		}
		return middleware.BackendError(&k, zone, dns.RcodeNameError, state, nil /* err */, middleware.Options{})
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if len(records) == 0 {
		return middleware.BackendError(&k, zone, dns.RcodeSuccess, state, nil, middleware.Options{})
	}

	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)

	m = dnsutil.Dedup(m)
	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (k Kubernetes) Name() string { return "kubernetes" }
