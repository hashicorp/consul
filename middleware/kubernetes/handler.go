package kubernetes

import (
	"fmt"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (k Kubernetes) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	if state.QClass() != dns.ClassINET {
		return dns.RcodeServerFailure, fmt.Errorf("can only deal with ClassINET")
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

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
		records, _, err = middleware.A(&k, zone, state, nil, middleware.Options{}) // Hmm wrt to '&k'
	case "AAAA":
		records, _, err = middleware.AAAA(&k, zone, state, nil, middleware.Options{})
	case "TXT":
		records, _, err = middleware.TXT(&k, zone, state, middleware.Options{})
	case "CNAME":
		records, _, err = middleware.CNAME(&k, zone, state, middleware.Options{})
	case "PTR":
		records, _, err = middleware.PTR(&k, zone, state, middleware.Options{})
	case "MX":
		records, extra, _, err = middleware.MX(&k, zone, state, middleware.Options{})
	case "SRV":
		records, extra, _, err = middleware.SRV(&k, zone, state, middleware.Options{})
	case "SOA":
		records, _, err = middleware.SOA(&k, zone, state, middleware.Options{})
	case "NS":
		if state.Name() == zone {
			records, extra, _, err = middleware.NS(&k, zone, state, middleware.Options{})
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, _, err = middleware.A(&k, zone, state, nil, middleware.Options{})
	}
	if k.IsNameError(err) {
		return middleware.BackendError(&k, zone, dns.RcodeNameError, state, nil /*debug*/, err, middleware.Options{})
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if len(records) == 0 {
		return middleware.BackendError(&k, zone, dns.RcodeSuccess, state, nil /*debug*/, nil, middleware.Options{})
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
