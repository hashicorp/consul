package forward

import (
	"crypto/tls"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin/pkg/up"

	"github.com/miekg/dns"
)

// Proxy defines an upstream host.
type Proxy struct {
	addr   string
	client *dns.Client

	// Connection caching
	expire    time.Duration
	transport *transport

	// health checking
	probe *up.Probe
	fails uint32

	avgRtt int64
}

// NewProxy returns a new proxy.
func NewProxy(addr string, tlsConfig *tls.Config) *Proxy {
	p := &Proxy{
		addr:      addr,
		fails:     0,
		probe:     up.New(),
		transport: newTransport(addr, tlsConfig),
		avgRtt:    int64(timeout / 2),
	}
	p.client = dnsClient(tlsConfig)
	return p
}

// dnsClient returns a client used for health checking.
func dnsClient(tlsConfig *tls.Config) *dns.Client {
	c := new(dns.Client)
	c.Net = "udp"
	// TODO(miek): this should be half of hcDuration?
	c.ReadTimeout = 1 * time.Second
	c.WriteTimeout = 1 * time.Second

	if tlsConfig != nil {
		c.Net = "tcp-tls"
		c.TLSConfig = tlsConfig
	}
	return c
}

// SetTLSConfig sets the TLS config in the lower p.transport.
func (p *Proxy) SetTLSConfig(cfg *tls.Config) { p.transport.SetTLSConfig(cfg) }

// SetExpire sets the expire duration in the lower p.transport.
func (p *Proxy) SetExpire(expire time.Duration) { p.transport.SetExpire(expire) }

// Dial connects to the host in p with the configured transport.
func (p *Proxy) Dial(proto string) (*dns.Conn, bool, error) { return p.transport.Dial(proto) }

// Yield returns the connection to the pool.
func (p *Proxy) Yield(c *dns.Conn) { p.transport.Yield(c) }

// Healthcheck kicks of a round of health checks for this proxy.
func (p *Proxy) Healthcheck() { p.probe.Do(p.Check) }

// Down returns true if this proxy is down, i.e. has *more* fails than maxfails.
func (p *Proxy) Down(maxfails uint32) bool {
	if maxfails == 0 {
		return false
	}

	fails := atomic.LoadUint32(&p.fails)
	return fails > maxfails
}

// close stops the health checking goroutine.
func (p *Proxy) close() {
	p.probe.Stop()
	p.transport.Stop()
}

// start starts the proxy's healthchecking.
func (p *Proxy) start(duration time.Duration) { p.probe.Start(duration) }

const (
	dialTimeout = 4 * time.Second
	timeout     = 2 * time.Second
	maxTimeout  = 2 * time.Second
	minTimeout  = 10 * time.Millisecond
	hcDuration  = 500 * time.Millisecond
)
