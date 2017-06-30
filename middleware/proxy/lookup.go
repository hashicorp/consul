package proxy

// functions other middleware might want to use to do lookup in the same style as the proxy.

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// NewLookup create a new proxy with the hosts in host and a Random policy.
func NewLookup(hosts []string) Proxy {
	return NewLookupWithOption(hosts, Options{})
}

// NewLookupWithOption process creates a simple round robin forward with potentially forced proto for upstream.
func NewLookupWithOption(hosts []string, opts Options) Proxy {
	p := Proxy{Next: nil}

	// TODO(miek): this needs to be unified with upstream.go's NewStaticUpstreams, caddy uses NewHost
	// we should copy/make something similar.
	upstream := &staticUpstream{
		from:        ".",
		Hosts:       make([]*UpstreamHost, len(hosts)),
		Policy:      &Random{},
		Spray:       nil,
		FailTimeout: 10 * time.Second,
		MaxFails:    3, // TODO(miek): disable error checking for simple lookups?
		Future:      60 * time.Second,
		ex:          newDNSExWithOption(opts),
	}

	for i, host := range hosts {
		uh := &UpstreamHost{
			Name:        host,
			Conns:       0,
			Fails:       0,
			FailTimeout: upstream.FailTimeout,

			CheckDown: func(upstream *staticUpstream) UpstreamHostDownFunc {
				return func(uh *UpstreamHost) bool {

					down := false

					uh.checkMu.Lock()
					until := uh.OkUntil
					uh.checkMu.Unlock()

					if !until.IsZero() && time.Now().After(until) {
						down = true
					}

					fails := atomic.LoadInt32(&uh.Fails)
					if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
						down = true
					}
					return down
				}
			}(upstream),
			WithoutPathPrefix: upstream.WithoutPathPrefix,
		}

		upstream.Hosts[i] = uh
	}
	p.Upstreams = &[]Upstream{upstream}
	return p
}

// Lookup will use name and type to forge a new message and will send that upstream. It will
// set any EDNS0 options correctly so that downstream will be able to process the reply.
func (p Proxy) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	req := new(dns.Msg)
	req.SetQuestion(name, typ)
	state.SizeAndDo(req)

	state2 := request.Request{W: state.W, Req: req}

	return p.lookup(state2)
}

// Forward forward the request in state as-is. Unlike Lookup that adds EDNS0 suffix to the message.
func (p Proxy) Forward(state request.Request) (*dns.Msg, error) {
	return p.lookup(state)
}

func (p Proxy) lookup(state request.Request) (*dns.Msg, error) {
	upstream := p.match(state)
	if upstream == nil {
		return nil, errInvalidDomain
	}
	for {
		start := time.Now()

		// Since Select() should give us "up" hosts, keep retrying
		// hosts until timeout (or until we get a nil host).
		for time.Now().Sub(start) < tryDuration {
			host := upstream.Select()
			if host == nil {
				return nil, errUnreachable
			}

			// duplicated from proxy.go, but with a twist, we don't write the
			// reply back to the client, we return it and there is no monitoring.

			atomic.AddInt64(&host.Conns, 1)

			reply, backendErr := upstream.Exchanger().Exchange(context.TODO(), host.Name, state)

			atomic.AddInt64(&host.Conns, -1)

			if backendErr == nil {
				return reply, nil
			}
			timeout := host.FailTimeout
			if timeout == 0 {
				timeout = 10 * time.Second
			}
			atomic.AddInt32(&host.Fails, 1)
			go func(host *UpstreamHost, timeout time.Duration) {
				time.Sleep(timeout)
				atomic.AddInt32(&host.Fails, -1)
			}(host, timeout)
		}
		return nil, errUnreachable
	}
}
