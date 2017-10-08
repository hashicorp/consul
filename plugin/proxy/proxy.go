// Package proxy is plugin that proxies requests.
package proxy

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/dnstap"
	"github.com/coredns/coredns/plugin/dnstap/msg"
	"github.com/coredns/coredns/plugin/pkg/healthcheck"
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

// Proxy represents a plugin instance that can proxy requests to another (DNS) server.
type Proxy struct {
	Next plugin.Handler

	// Upstreams is a pointer to a slice, so we can update the upstream (used for Google)
	// midway.

	Upstreams *[]Upstream

	// Trace is the Trace plugin, if it is installed
	// This is used by the grpc exchanger to trace through the grpc calls
	Trace plugin.Handler
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
var tryDuration = 16 * time.Second

// ServeDNS satisfies the plugin.Handler interface.
func (p Proxy) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	var span, child ot.Span
	span = ot.SpanFromContext(ctx)
	state := request.Request{W: w, Req: r}

	upstream := p.match(state)
	if upstream == nil {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
	}

	for {
		start := time.Now()
		reply := new(dns.Msg)
		var backendErr error

		// Since Select() should give us "up" hosts, keep retrying
		// hosts until timeout (or until we get a nil host).
		for time.Since(start) < tryDuration {
			host := upstream.Select()
			if host == nil {
				return dns.RcodeServerFailure, fmt.Errorf("%s: %s", errUnreachable, "no upstream host")
			}

			if span != nil {
				child = span.Tracer().StartSpan("exchange", ot.ChildOf(span.Context()))
				ctx = ot.ContextWithSpan(ctx, child)
			}

			atomic.AddInt64(&host.Conns, 1)
			queryEpoch := msg.Epoch()

			RequestCount.WithLabelValues(state.Proto(), upstream.Exchanger().Protocol(), familyToString(state.Family()), host.Name).Add(1)

			reply, backendErr = upstream.Exchanger().Exchange(ctx, host.Name, state)

			respEpoch := msg.Epoch()
			atomic.AddInt64(&host.Conns, -1)

			if child != nil {
				child.Finish()
			}

			taperr := toDnstap(ctx, host.Name, upstream.Exchanger(), state, reply, queryEpoch, respEpoch)

			if backendErr == nil {
				w.WriteMsg(reply)

				RequestDuration.WithLabelValues(state.Proto(), upstream.Exchanger().Protocol(), familyToString(state.Family()), host.Name).Observe(float64(time.Since(start) / time.Millisecond))

				return 0, taperr
			}

			// A "ANY isc.org" query is being dropped by ISC's nameserver, we see this as a i/o timeout, but
			// would then mark our upstream is being broken. We should not do this if we consider the error temporary.
			// Of course it could really be that our upstream is broken
			if oe, ok := backendErr.(*net.OpError); ok {
				// Note this keeps looping and trying until tryDuration is hit, at which point our client
				// might be long gone...
				if oe.Timeout() {
					// Our upstream's upstream is problably messing up, continue with next selected
					// host - which my be the *same* one as we don't set any uh.Fails.
					continue
				}
			}

			timeout := host.FailTimeout
			if timeout == 0 {
				timeout = 2 * time.Second
			}

			atomic.AddInt32(&host.Fails, 1)

			go func(host *healthcheck.UpstreamHost, timeout time.Duration) {
				time.Sleep(timeout)
				atomic.AddInt32(&host.Fails, -1)
			}(host, timeout)
		}

		return dns.RcodeServerFailure, fmt.Errorf("%s: %s", errUnreachable, backendErr)
	}
}

func (p Proxy) match(state request.Request) (u Upstream) {
	if p.Upstreams == nil {
		return nil
	}

	longestMatch := 0
	for _, upstream := range *p.Upstreams {
		from := upstream.From()

		if !plugin.Name(from).Matches(state.Name()) || !upstream.IsAllowedDomain(state.Name()) {
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
