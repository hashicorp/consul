package etcd

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (e Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	println("ETCD MIDDLEWARE HIT")

	state := middleware.State{W: w, Req: r}

	m := state.AnswerMessage()
	m.Authoritative = true
	m.RecursionAvailable = true
	m.Compress = true

	// TODO(miek): get current zone when serving multiple
	zone := "."

	switch state.Type() {
	case "A":
		records, err := e.A(zone, state, nil)
	case "AAAA":
		records, err := e.AAAA(zone, state, nil)
		fallthrough
	case "TXT":
		records, err := e.TXT(zone, state)
		fallthrough
	case "CNAME":
		records, err := e.CNAME(zone, state)
		fallthrough
	case "MX":
		records, extra, err := e.MX(zone, state)
		fallthrough
	case "SRV":
		records, extra, err := e.SRV(zone, state)
		if isEtcdNameError(err) {
			NameError(zone, state)
			return dns.RcodeNameError, nil
		}
		if err != nil {
			// TODO(miek): err or nil in this case?
			return dns.RcodeServerFailure, err
		}
		if len(records) > 0 {
			m.Answer = append(m.Answer, records...)
		}
		if len(extra) > 0 {
			m.Extra = append(m.Extra, extra...)
		}
	default:
		// Nodata response
		// also catch other types, so that they return NODATA
	}
	return e.Next.ServeDNS(ctx, w, r)
}

// NameError writes a name error to the client.
func NameError(zone string, state middleware.State) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, dns.RcodeNameError)

	m.Ns = []dns.RR{NewSOA()}
	m.Ns[0].Header().Ttl = minTtl

	state.W.WriteMsg(m)
}

// NoData write a nodata response to the client.
func NoData(zone string, state middleware.State) {
	// TODO(miek): write it
}
