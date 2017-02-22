// Package erratic implements a middleware that returns erratic answers (delayed, dropped).
package erratic

import (
	"sync/atomic"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Erratic is a middleware that returns erratic repsonses to each client.
type Erratic struct {
	amount uint64

	q uint64 // counter of queries
}

// ServeDNS implements the middleware.Handler interface.
func (e *Erratic) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	drop := false
	if e.amount > 0 {
		queryNr := atomic.LoadUint64(&e.q)

		if queryNr%e.amount == 0 {
			drop = true
		}

		atomic.AddUint64(&e.q, 1)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = true
	m.Authoritative = true

	// small dance to copy rrA or rrAAAA into a non-pointer var that allows us to overwrite the ownername
	// in a non-racy manor.
	switch state.QType() {
	case dns.TypeA:
		rr := *(rrA.(*dns.A))
		rr.Header().Name = state.QName()
		m.Answer = append(m.Answer, &rr)
	case dns.TypeAAAA:
		rr := *(rrAAAA.(*dns.AAAA))
		rr.Header().Name = state.QName()
		m.Answer = append(m.Answer, &rr)
	default:
		if !drop {
			// coredns will return error.
			return dns.RcodeServerFailure, nil
		}
	}

	if drop {
		return 0, nil
	}

	state.SizeAndDo(m)
	w.WriteMsg(m)

	return 0, nil
}

// Name implements the Handler interface.
func (e *Erratic) Name() string { return "erratic" }

var (
	rrA, _    = dns.NewRR(". IN 0 A 192.0.2.53")
	rrAAAA, _ = dns.NewRR(". IN 0 AAAA 2001:DB8::53")
)
