// Package proxy is plugin that proxies requests.
package proxy

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/healthcheck"
	"github.com/coredns/coredns/request"

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

			RequestCount.WithLabelValues(state.Proto(), upstream.Exchanger().Protocol(), familyToString(state.Family()), host.Name).Add(1)

			reply, backendErr = upstream.Exchanger().Exchange(ctx, host.Name, state)

			atomic.AddInt64(&host.Conns, -1)

			if child != nil {
				child.Finish()
			}

			taperr := toDnstap(ctx, host.Name, upstream.Exchanger(), state, reply, start)

			if backendErr == nil {
				w.WriteMsg(reply)

				RequestDuration.WithLabelValues(state.Proto(), upstream.Exchanger().Protocol(), familyToString(state.Family()), host.Name).Observe(time.Since(start).Seconds())

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

			// If protocol is https_google we do the health checks wrong, i.e. we're healthchecking the wrong
			// endpoint, hence the health check code below should not be executed. See issue #1202.
			// This is an ugly hack and the thing requires a rethink. Possibly in conjunction with moving
			// to the *forward* plugin.
			if upstream.Exchanger().Protocol() == "https_google" {
				continue
			}

			timeout := host.FailTimeout
			if timeout == 0 {
				timeout = defaultFailTimeout
			}

			atomic.AddInt32(&host.Fails, 1)
			fails := atomic.LoadInt32(&host.Fails)

			go func(host *healthcheck.UpstreamHost, timeout time.Duration) {
				time.Sleep(timeout)
				// we may go negative here, should be rectified by the HC.
				atomic.AddInt32(&host.Fails, -1)
				if fails%failureCheck == 0 { // Kick off healthcheck on eveyry third failure.
					host.HealthCheckURL()
				}
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

const (
	defaultFailTimeout = 2 * time.Second
	defaultTimeout     = 5 * time.Second
	failureCheck       = 3
)
