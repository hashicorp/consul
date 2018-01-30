package proxy

// functions other plugin might want to use to do lookup in the same style as the proxy.

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin/pkg/healthcheck"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// NewLookup create a new proxy with the hosts in host and a Random policy.
func NewLookup(hosts []string) Proxy { return NewLookupWithOption(hosts, Options{}) }

// NewLookupWithOption process creates a simple round robin forward with potentially forced proto for upstream.
func NewLookupWithOption(hosts []string, opts Options) Proxy {
	p := Proxy{Next: nil}

	// TODO(miek): this needs to be unified with upstream.go's NewStaticUpstreams, caddy uses NewHost
	// we should copy/make something similar.
	upstream := &staticUpstream{
		from: ".",
		HealthCheck: healthcheck.HealthCheck{
			FailTimeout: 5 * time.Second,
			MaxFails:    3,
		},
		ex: newDNSExWithOption(opts),
	}
	upstream.Hosts = make([]*healthcheck.UpstreamHost, len(hosts))

	for i, host := range hosts {
		uh := &healthcheck.UpstreamHost{
			Name:        host,
			FailTimeout: upstream.FailTimeout,
			CheckDown:   checkDownFunc(upstream),
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
		reply := new(dns.Msg)
		var backendErr error

		// Since Select() should give us "up" hosts, keep retrying
		// hosts until timeout (or until we get a nil host).
		for time.Since(start) < tryDuration {
			host := upstream.Select()
			if host == nil {
				return nil, fmt.Errorf("%s: %s", errUnreachable, "no upstream host")
			}

			// duplicated from proxy.go, but with a twist, we don't write the
			// reply back to the client, we return it and there is no monitoring to update here.

			atomic.AddInt64(&host.Conns, 1)

			reply, backendErr = upstream.Exchanger().Exchange(context.TODO(), host.Name, state)

			atomic.AddInt64(&host.Conns, -1)

			if backendErr == nil {
				return reply, nil
			}

			if oe, ok := backendErr.(*net.OpError); ok {
				if oe.Timeout() { // see proxy.go for docs.
					continue
				}
			}

			timeout := host.FailTimeout
			if timeout == 0 {
				timeout = defaultFailTimeout
			}

			atomic.AddInt32(&host.Fails, 1)
			fails := atomic.LoadInt32(&host.Fails)

			go func(host *healthcheck.UpstreamHost, timeout time.Duration) {
				time.Sleep(timeout)
				atomic.AddInt32(&host.Fails, -1)
				if fails%failureCheck == 0 { // Kick off healthcheck on eveyry third failure.
					host.HealthCheckURL()
				}
			}(host, timeout)
		}
		return nil, fmt.Errorf("%s: %s", errUnreachable, backendErr)
	}
}
