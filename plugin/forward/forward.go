// Package forward implements a forwarding proxy. It caches an upstream net.Conn for some time, so if the same
// client returns the upstream's Conn will be precached. Depending on how you benchmark this looks to be
// 50% faster than just openening a new connection for every client. It works with UDP and TCP and uses
// inband healthchecking.
package forward

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

// Forward represents a plugin instance that can proxy requests to another (DNS) server. It has a list
// of proxies each representing one upstream proxy.
type Forward struct {
	proxies    []*Proxy
	p          Policy
	hcInterval time.Duration

	from    string
	ignored []string

	tlsConfig     *tls.Config
	tlsServerName string
	maxfails      uint32
	expire        time.Duration

	forceTCP bool // also here for testing

	Next plugin.Handler
}

// New returns a new Forward.
func New() *Forward {
	f := &Forward{maxfails: 2, tlsConfig: new(tls.Config), expire: defaultExpire, p: new(random), from: ".", hcInterval: hcDuration}
	return f
}

// SetProxy appends p to the proxy list and starts healthchecking.
func (f *Forward) SetProxy(p *Proxy) {
	f.proxies = append(f.proxies, p)
	p.start(f.hcInterval)
}

// Len returns the number of configured proxies.
func (f *Forward) Len() int { return len(f.proxies) }

// Name implements plugin.Handler.
func (f *Forward) Name() string { return "forward" }

// ServeDNS implements plugin.Handler.
func (f *Forward) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	state := request.Request{W: w, Req: r}
	if !f.match(state) {
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, w, r)
	}

	fails := 0
	var span, child ot.Span
	var upstreamErr error
	span = ot.SpanFromContext(ctx)
	i := 0
	list := f.list()
	deadline := time.Now().Add(defaultTimeout)

	for time.Now().Before(deadline) {
		if i >= len(list) {
			// reached the end of list, reset to begin
			i = 0
			fails = 0
		}

		proxy := list[i]
		i++
		if proxy.Down(f.maxfails) {
			fails++
			if fails < len(f.proxies) {
				continue
			}
			// All upstream proxies are dead, assume healtcheck is completely broken and randomly
			// select an upstream to connect to.
			r := new(random)
			proxy = r.List(f.proxies)[0]

			HealthcheckBrokenCount.Add(1)
		}

		if span != nil {
			child = span.Tracer().StartSpan("connect", ot.ChildOf(span.Context()))
			ctx = ot.ContextWithSpan(ctx, child)
		}

		var (
			ret *dns.Msg
			err error
		)
		for {
			ret, err = proxy.connect(ctx, state, f.forceTCP, true)
			if err != nil && err == errCachedClosed { // Remote side closed conn, can only happen with TCP.
				continue
			}
			break
		}

		if child != nil {
			child.Finish()
		}

		ret, err = truncated(state, ret, err)
		upstreamErr = err

		if err != nil {
			// Kick off health check to see if *our* upstream is broken.
			if f.maxfails != 0 {
				proxy.Healthcheck()
			}

			if fails < len(f.proxies) {
				continue
			}
			break
		}

		// Check if the reply is correct; if not return FormErr.
		if !state.Match(ret) {
			formerr := state.ErrorMessage(dns.RcodeFormatError)
			w.WriteMsg(formerr)
			return 0, nil
		}

		ret.Compress = true
		// When using force_tcp the upstream can send a message that is too big for
		// the udp buffer, hence we need to truncate the message to at least make it
		// fit the udp buffer.
		ret, _ = state.Scrub(ret)

		w.WriteMsg(ret)

		return 0, nil
	}

	if upstreamErr != nil {
		return dns.RcodeServerFailure, upstreamErr
	}

	return dns.RcodeServerFailure, errNoHealthy
}

func (f *Forward) match(state request.Request) bool {
	from := f.from

	if !plugin.Name(from).Matches(state.Name()) || !f.isAllowedDomain(state.Name()) {
		return false
	}

	return true
}

func (f *Forward) isAllowedDomain(name string) bool {
	if dns.Name(name) == dns.Name(f.from) {
		return true
	}

	for _, ignore := range f.ignored {
		if plugin.Name(ignore).Matches(name) {
			return false
		}
	}
	return true
}

// List returns a set of proxies to be used for this client depending on the policy in f.
func (f *Forward) list() []*Proxy { return f.p.List(f.proxies) }

var (
	errInvalidDomain = errors.New("invalid domain for forward")
	errNoHealthy     = errors.New("no healthy proxies")
	errNoForward     = errors.New("no forwarder defined")
	errCachedClosed  = errors.New("cached connection was closed by peer")
)

// policy tells forward what policy for selecting upstream it uses.
type policy int

const (
	randomPolicy policy = iota
	roundRobinPolicy
	sequentialPolicy
)

const defaultTimeout = 5 * time.Second
