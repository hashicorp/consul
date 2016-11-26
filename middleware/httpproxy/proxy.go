// Package httpproxy is middleware that proxies requests to a HTTPs server doing DNS.
package httpproxy

import (
	"errors"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var errUnreachable = errors.New("unreachable backend")

// Proxy represents a middleware instance that can proxy requests to HTTPS servers.
type Proxy struct {
	from string
	e    Exchanger

	Next middleware.Handler
}

// ServeDNS satisfies the middleware.Handler interface.
func (p *Proxy) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	start := time.Now()
	state := request.Request{W: w, Req: r}

	reply, backendErr := p.e.Exchange(r)

	if backendErr == nil {
		state.SizeAndDo(reply)

		w.WriteMsg(reply)
		RequestDuration.WithLabelValues(p.from).Observe(float64(time.Since(start) / time.Millisecond))
		return 0, nil
	}
	RequestDuration.WithLabelValues(p.from).Observe(float64(time.Since(start) / time.Millisecond))

	return dns.RcodeServerFailure, errUnreachable
}

// Name implements the Handler interface.
func (p Proxy) Name() string { return "httpproxy" }
