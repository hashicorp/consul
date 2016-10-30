package proxy

// functions other middleware might want to use to do lookup in the same style as the proxy.

import (
	"sync/atomic"
	"time"

	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

// New create a new proxy with the hosts in host and a Random policy.
func New(hosts []string) Proxy {
	p := Proxy{Next: nil, Client: newClient()}

	upstream := &staticUpstream{
		from:        "",
		Hosts:       make([]*UpstreamHost, len(hosts)),
		Policy:      &Random{},
		Spray:       nil,
		FailTimeout: 10 * time.Second,
		MaxFails:    1,
	}

	for i, host := range hosts {
		uh := &UpstreamHost{
			Name:        host,
			Conns:       0,
			Fails:       0,
			FailTimeout: upstream.FailTimeout,

			Unhealthy: false,
			CheckDown: func(upstream *staticUpstream) UpstreamHostDownFunc {
				return func(uh *UpstreamHost) bool {
					if uh.Unhealthy {
						return true
					}
					fails := atomic.LoadInt32(&uh.Fails)
					if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
						return true
					}
					return false
				}
			}(upstream),
			WithoutPathPrefix: upstream.WithoutPathPrefix,
		}
		upstream.Hosts[i] = uh
	}
	p.Upstreams = []Upstream{upstream}
	return p
}

// Lookup will use name and type to forge a new message and will send that upstream. It will
// set any EDNS0 options correctly so that downstream will be able to process the reply.
func (p Proxy) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	req := new(dns.Msg)
	req.SetQuestion(name, typ)
	state.SizeAndDo(req)

	return p.lookup(state, req)
}

// Forward forward the request in state as-is. Unlike Lookup that adds EDNS0 suffix to the message.
func (p Proxy) Forward(state request.Request) (*dns.Msg, error) {
	return p.lookup(state, state.Req)
}

func (p Proxy) lookup(state request.Request, r *dns.Msg) (*dns.Msg, error) {
	for _, upstream := range p.Upstreams {
		start := time.Now()

		// Since Select() should give us "up" hosts, keep retrying
		// hosts until timeout (or until we get a nil host).
		for time.Now().Sub(start) < tryDuration {
			host := upstream.Select()
			if host == nil {
				return nil, errUnreachable
			}

			// duplicated from proxy.go, but with a twist, we don't write the
			// reply back to the client, we return it.

			atomic.AddInt64(&host.Conns, 1)

			reply, backendErr := p.Client.ServeDNS(state.W, r, host)

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
	return nil, errUnreachable
}
