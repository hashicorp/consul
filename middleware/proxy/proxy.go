// Package proxy is middleware that proxies requests.
package proxy

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/dnstap"
	"github.com/coredns/coredns/middleware/dnstap/msg"
	"github.com/coredns/coredns/middleware/pkg/healthcheck"
	"github.com/coredns/coredns/request"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

var (
	errUnreachable     = errors.New("unreachable backend")
	errInvalidProtocol = errors.New("invalid protocol")
	errInvalidDomain   = errors.New("invalid path for proxy")
)

// Proxy represents a middleware instance that can proxy requests to another (DNS) server.
type Proxy struct {
	Next middleware.Handler

	// Upstreams is a pointer to a slice, so we can update the upstream (used for Google)
	// midway.

	Upstreams *[]Upstream

	// Trace is the Trace middleware, if it is installed
	// This is used by the grpc exchanger to trace through the grpc calls
	Trace middleware.Handler
}

// Upstream manages a pool of proxy upstream hosts. Select should return a
// suitable upstream host, or nil if no such hosts are available.
type Upstream interface {
	// The domain name this upstream host should be routed on.
	From() string
	// Selects an upstream host to be routed to.
	Select() *healthcheck.UpstreamHost
	// Checks if subpdomain is not an ignored.
	IsAllowedDomain(string) bool
	// Exchanger returns the exchanger to be used for this upstream.
	Exchanger() Exchanger
	// Stops the upstream from proxying requests to shutdown goroutines cleanly.
	Stop() error
}

// tryDuration is how long to try upstream hosts; failures result in
// immediate retries until this duration ends or we get a nil host.
var tryDuration = 60 * time.Second

// ServeDNS satisfies the middleware.Handler interface.
func (p Proxy) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	var span, child ot.Span
	span = ot.SpanFromContext(ctx)
	state := request.Request{W: w, Req: r}

	upstream := p.match(state)
	if upstream == nil {
		return middleware.NextOrFailure(p.Name(), p.Next, ctx, w, r)
	}

	for {
		start := time.Now()

		// Since Select() should give us "up" hosts, keep retrying
		// hosts until timeout (or until we get a nil host).
		for time.Since(start) < tryDuration {
			host := upstream.Select()
			if host == nil {

				RequestDuration.WithLabelValues(state.Proto(), upstream.Exchanger().Protocol(), upstream.From()).Observe(float64(time.Since(start) / time.Millisecond))

				return dns.RcodeServerFailure, errUnreachable
			}

			if span != nil {
				child = span.Tracer().StartSpan("exchange", ot.ChildOf(span.Context()))
				ctx = ot.ContextWithSpan(ctx, child)
			}

			atomic.AddInt64(&host.Conns, 1)
			queryEpoch := msg.Epoch()

			reply, backendErr := upstream.Exchanger().Exchange(ctx, host.Name, state)

			respEpoch := msg.Epoch()
			atomic.AddInt64(&host.Conns, -1)

			if child != nil {
				child.Finish()
			}

			taperr := toDnstap(ctx, host.Name, upstream.Exchanger(), state, reply,
				queryEpoch, respEpoch)

			if backendErr == nil {
				w.WriteMsg(reply)

				RequestDuration.WithLabelValues(state.Proto(), upstream.Exchanger().Protocol(), upstream.From()).Observe(float64(time.Since(start) / time.Millisecond))

				return 0, taperr
			}

			timeout := host.FailTimeout
			if timeout == 0 {
				timeout = 10 * time.Second
			}
			atomic.AddInt32(&host.Fails, 1)
			go func(host *healthcheck.UpstreamHost, timeout time.Duration) {
				time.Sleep(timeout)
				atomic.AddInt32(&host.Fails, -1)
			}(host, timeout)
		}

		RequestDuration.WithLabelValues(state.Proto(), upstream.Exchanger().Protocol(), upstream.From()).Observe(float64(time.Since(start) / time.Millisecond))

		return dns.RcodeServerFailure, errUnreachable
	}
}

func (p Proxy) match(state request.Request) (u Upstream) {
	if p.Upstreams == nil {
		return nil
	}

	longestMatch := 0
	for _, upstream := range *p.Upstreams {
		from := upstream.From()

		if !middleware.Name(from).Matches(state.Name()) || !upstream.IsAllowedDomain(state.Name()) {
			continue
		}

		if lf := len(from); lf > longestMatch {
			longestMatch = lf
			u = upstream
		}
	}
	return u

}

// Name implements the Handler interface.
func (p Proxy) Name() string { return "proxy" }

// defaultTimeout is the default networking timeout for DNS requests.
const defaultTimeout = 5 * time.Second

func toDnstap(ctx context.Context, host string, ex Exchanger, state request.Request, reply *dns.Msg, queryEpoch, respEpoch uint64) (err error) {
	if tapper := dnstap.TapperFromContext(ctx); tapper != nil {
		// Query
		b := tapper.TapBuilder()
		b.TimeSec = queryEpoch
		if err = b.HostPort(host); err != nil {
			return
		}
		t := ex.Transport()
		if t == "" {
			t = state.Proto()
		}
		if t == "tcp" {
			b.SocketProto = tap.SocketProtocol_TCP
		} else {
			b.SocketProto = tap.SocketProtocol_UDP
		}
		if err = b.Msg(state.Req); err != nil {
			return
		}
		err = tapper.TapMessage(b.ToOutsideQuery(tap.Message_FORWARDER_QUERY))
		if err != nil {
			return
		}

		// Response
		if reply != nil {
			b.TimeSec = respEpoch
			if err = b.Msg(reply); err != nil {
				return
			}
			err = tapper.TapMessage(b.ToOutsideResponse(tap.Message_FORWARDER_RESPONSE))
		}
	}
	return
}
