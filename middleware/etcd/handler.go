package etcd

import (
	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (e Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	println("ETCD MIDDLEWARE HIT")
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
		return 0, nil
	}
	if isEtcdNameError(err) {
		NameError(zone, state)
		return dns.RcodeNameError, nil
	}
	if err != nil {
		println(err.Error())
		// TODO(miek): err or nil in this case?
		return dns.RcodeServerFailure, err
	}
	if len(records) > 0 {
		m.Answer = append(m.Answer, records...)
	}
	if len(extra) > 0 {
		m.Extra = append(m.Extra, extra...)
	}
	state.W.WriteMsg(m)
	return 0, nil
}

// NameError writes a name error to the client.
func NameError(zone string, state middleware.State) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, dns.RcodeNameError)
	m.Ns = []dns.RR{SOA(zone)}
	state.W.WriteMsg(m)
}

// NoData write a nodata response to the client.
func NoData(zone string, state middleware.State) {
	// TODO(miek): write it
}
