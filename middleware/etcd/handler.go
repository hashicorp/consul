package etcd

import (
	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (e Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}
	zone := middleware.Zones(e.Zones).Matches(state.Name())
	if zone == "" {
		return e.Next.ServeDNS(ctx, w, r)
	}

	m := state.AnswerMessage()
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
		m := new(dns.Msg)
		m.SetRcode(state.Req, dns.RcodeNameError)
		m.Ns = []dns.RR{e.SOA(zone, state)}
		state.W.WriteMsg(m)
		return dns.RcodeNameError, nil
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if len(records) == 0 {
		// NODATE function, see below
		m := new(dns.Msg)
		m.SetReply(state.Req)
		m.Ns = []dns.RR{e.SOA(zone, state)}
		state.W.WriteMsg(m)
		return dns.RcodeSuccess, nil

	}
	if len(records) > 0 {
		m.Answer = append(m.Answer, records...)
	}
	if len(extra) > 0 {
		m.Extra = append(m.Extra, extra...)
	}

	m = dedup(m)
	m, _ = state.Scrub(m)
	state.W.WriteMsg(m)
	return 0, nil
}

// NoData write a nodata response to the client.
func (e Etcd) NoData(zone string, state middleware.State) {
	// TODO(miek): write it
}

func dedup(m *dns.Msg) *dns.Msg {
	ma := make(map[string]dns.RR)
	m.Answer = dns.Dedup(m.Answer, ma)
	m.Ns = dns.Dedup(m.Ns, ma)
	m.Extra = dns.Dedup(m.Extra, ma)
	return m
}
