package etcd

import (
	"fmt"
	"strings"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (e Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}
	if state.QClass() != dns.ClassINET {
		return dns.RcodeServerFailure, fmt.Errorf("can only deal with ClassINET")
	}

	// We need to check stubzones first, because we may get a request for a zone we
	// are not auth. for *but* do have a stubzone forward for. If we do the stubzone
	// handler will handle the request.
	name := state.Name()
	if e.Stubmap != nil && len(*e.Stubmap) > 0 {
		for zone, _ := range *e.Stubmap {
			// TODO(miek): use the Match function.
			if strings.HasSuffix(name, zone) {
				stub := Stub{Etcd: e, Zone: zone}
				return stub.ServeDNS(ctx, w, r)
			}
		}
	}

	zone := middleware.Zones(e.Zones).Matches(state.Name())
	if zone == "" {
		return e.Next.ServeDNS(ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	var (
		records, extra []dns.RR
		err            error
	)
	switch state.Type() {
	case "A":
		records, err = e.A(zone, state, nil)
	case "AAAA":
		records, err = e.AAAA(zone, state, nil)
	case "TXT":
		records, err = e.TXT(zone, state)
	case "CNAME":
		records, err = e.CNAME(zone, state)
	case "MX":
		records, extra, err = e.MX(zone, state)
	case "SRV":
		records, extra, err = e.SRV(zone, state)
	default:
		// For SOA and NS we might still want this
		// and use dns.<zones> as the name to put these
		// also for stub
		// rwrite and return
		// Nodata response
		// also catch other types, so that they return NODATA
		// TODO(miek) nodata function see below
		return 0, nil
	}
	if isEtcdNameError(err) {
		return e.Err(zone, dns.RcodeNameError, state)
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if len(records) == 0 {
		return e.Err(zone, dns.RcodeSuccess, state)
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
func (e Etcd) Err(zone string, rcode int, state middleware.State) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Ns = []dns.RR{e.SOA(zone, state)}
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
