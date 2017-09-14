// Package erratic implements a plugin that returns erratic answers (delayed, dropped).
package erratic

import (
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Erratic is a plugin that returns erratic repsonses to each client.
type Erratic struct {
	drop uint64

	delay    uint64
	duration time.Duration

	truncate uint64

	q uint64 // counter of queries
}

// ServeDNS implements the plugin.Handler interface.
func (e *Erratic) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	drop := false
	delay := false
	trunc := false

	queryNr := atomic.LoadUint64(&e.q)
	atomic.AddUint64(&e.q, 1)

	if e.drop > 0 && queryNr%e.drop == 0 {
		drop = true
	}
	if e.delay > 0 && queryNr%e.delay == 0 {
		delay = true
	}
	if e.truncate > 0 && queryNr&e.truncate == 0 {
		trunc = true
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = true
	m.Authoritative = true
	if trunc {
		m.Truncated = true
	}

	// small dance to copy rrA or rrAAAA into a non-pointer var that allows us to overwrite the ownername
	// in a non-racy way.
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
			if delay {
				time.Sleep(e.duration)
			}
			// coredns will return error.
			return dns.RcodeServerFailure, nil
		}
	}

	if drop {
		return 0, nil
	}

	if delay {
		time.Sleep(e.duration)
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
