// Package proxy is middleware that proxies requests.
package proxy

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

var errUnreachable = errors.New("unreachable backend")

// Proxy represents a middleware instance that can proxy requests.
type Proxy struct {
	Next      middleware.Handler
	Client    Client
	Upstreams []Upstream
}

type Client struct {
	UDP *dns.Client
	TCP *dns.Client
}

// Upstream manages a pool of proxy upstream hosts. Select should return a
// suitable upstream host, or nil if no such hosts are available.
type Upstream interface {
	// The domain name this upstream host should be routed on.
	From() string
	// Selects an upstream host to be routed to.
	Select() *UpstreamHost
	// Checks if subpdomain is not an ignored.
	IsAllowedPath(string) bool
}

// UpstreamHostDownFunc can be used to customize how Down behaves.
type UpstreamHostDownFunc func(*UpstreamHost) bool

// UpstreamHost represents a single proxy upstream
type UpstreamHost struct {
	Conns             int64  // must be first field to be 64-bit aligned on 32-bit systems
	Name              string // IP address (and port) of this upstream host
	Fails             int32
	FailTimeout       time.Duration
	Unhealthy         bool
	ExtraHeaders      http.Header
	CheckDown         UpstreamHostDownFunc
	WithoutPathPrefix string
}

// Down checks whether the upstream host is down or not.
// Down will try to use uh.CheckDown first, and will fall
// back to some default criteria if necessary.
func (uh *UpstreamHost) Down() bool {
	if uh.CheckDown == nil {
		// Default settings
		return uh.Unhealthy || uh.Fails > 0
	}
	return uh.CheckDown(uh)
}

// tryDuration is how long to try upstream hosts; failures result in
// immediate retries until this duration ends or we get a nil host.
var tryDuration = 60 * time.Second

// ServeDNS satisfies the middleware.Handler interface.
func (p Proxy) ServeDNS(w dns.ResponseWriter, r *dns.Msg) (int, error) {
	for _, upstream := range p.Upstreams {
		// allowed bla bla bla TODO(miek): fix full proxy spec from caddy
		start := time.Now()

		// Since Select() should give us "up" hosts, keep retrying
		// hosts until timeout (or until we get a nil host).
		for time.Now().Sub(start) < tryDuration {
			host := upstream.Select()
			if host == nil {
				return dns.RcodeServerFailure, errUnreachable
			}
			// TODO(miek): PORT!
			reverseproxy := ReverseProxy{Host: host.Name, Client: p.Client}

			atomic.AddInt64(&host.Conns, 1)
			backendErr := reverseproxy.ServeDNS(w, r, nil)
			atomic.AddInt64(&host.Conns, -1)
			if backendErr == nil {
				return 0, nil
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
		return dns.RcodeServerFailure, errUnreachable
	}
	return p.Next.ServeDNS(w, r)
}

func Clients() Client {
	udp := newClient("udp", defaultTimeout)
	tcp := newClient("tcp", defaultTimeout)
	return Client{UDP: udp, TCP: tcp}
}

// newClient returns a new client for proxy requests.
func newClient(net string, timeout time.Duration) *dns.Client {
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &dns.Client{Net: net, ReadTimeout: timeout, WriteTimeout: timeout, SingleInflight: true}
}

const defaultTimeout = 5 * time.Second
