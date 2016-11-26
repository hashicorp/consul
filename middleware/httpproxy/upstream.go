package httpproxy

import (
	"sync/atomic"
	"time"

	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	"github.com/miekg/coredns/middleware/proxy"
)

type simpleUpstream struct {
	from   string
	Hosts  proxy.HostPool
	Policy proxy.Policy

	FailTimeout time.Duration
	MaxFails    int32
}

// newSimpleUpstream return a new simpleUpstream initialized with the addresses.
func newSimpleUpstream(hosts []string) (*simpleUpstream, error) {
	upstream := &simpleUpstream{
		Hosts:       nil,
		Policy:      &proxy.Random{},
		FailTimeout: 10 * time.Second,
		MaxFails:    3,
	}

	toHosts, err := dnsutil.ParseHostPortOrFile(hosts...)
	if err != nil {
		return upstream, err
	}

	upstream.Hosts = make([]*proxy.UpstreamHost, len(toHosts))
	for i, host := range toHosts {
		uh := &proxy.UpstreamHost{
			Name:        host,
			Conns:       0,
			Fails:       0,
			FailTimeout: upstream.FailTimeout,
			Unhealthy:   false,

			CheckDown: func(upstream *simpleUpstream) proxy.UpstreamHostDownFunc {
				return func(uh *proxy.UpstreamHost) bool {
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
		}
		upstream.Hosts[i] = uh
	}
	return upstream, nil
}

func (u *simpleUpstream) From() string                   { return u.from }
func (u *simpleUpstream) Options() proxy.Options         { return proxy.Options{} }
func (u *simpleUpstream) IsAllowedPath(name string) bool { return true }

func (u *simpleUpstream) Select() *proxy.UpstreamHost {
	pool := u.Hosts
	if len(pool) == 1 {
		if pool[0].Down() {
			return nil
		}
		return pool[0]
	}
	allDown := true
	for _, host := range pool {
		if !host.Down() {
			allDown = false
			break
		}
	}
	if allDown {
		return nil
	}

	if u.Policy == nil {
		h := (&proxy.Random{}).Select(pool)
		return h
	}

	h := u.Policy.Select(pool)
	return h
}
