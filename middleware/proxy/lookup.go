package proxy

// functions OTHER middleware might want to use to do lookup in the same
// style as the proxy.

import (
	"sync/atomic"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

// New create a new proxy with the hosts in host and a Random policy.
func New(hosts []string) Proxy {
	p := Proxy{Next: nil, Client: Clients()}

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
			Unhealthy:   false,
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
// Lookup is not suitable for forwarding request. Ssee for that.
func (p Proxy) Lookup(state middleware.State, name string, tpe uint16) (*dns.Msg, error) {
	req := new(dns.Msg)
	req.SetQuestion(name, tpe)
	state.SizeAndDo(req)
	return p.lookup(state, req)
}

func (p Proxy) Forward(state middleware.State) (*dns.Msg, error) {
	return p.lookup(state, state.Req)
}

func (p Proxy) lookup(state middleware.State, r *dns.Msg) (*dns.Msg, error) {
	var (
		reply *dns.Msg
		err   error
	)
	for _, upstream := range p.Upstreams {
		// allowed bla bla bla TODO(miek): fix full proxy spec from caddy?
		start := time.Now()

		// Since Select() should give us "up" hosts, keep retrying
		// hosts until timeout (or until we get a nil host).
		for time.Now().Sub(start) < tryDuration {
			host := upstream.Select()
			if host == nil {
				return nil, errUnreachable
			}

			atomic.AddInt64(&host.Conns, 1)
			if state.Proto() == "tcp" {
				reply, err = middleware.Exchange(p.Client.TCP, r, host.Name)
			} else {
				reply, err = middleware.Exchange(p.Client.UDP, r, host.Name)
			}
			atomic.AddInt64(&host.Conns, -1)

			if err == nil {
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
