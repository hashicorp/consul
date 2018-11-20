package forward

import (
	"crypto/tls"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin/pkg/up"
)

// Proxy defines an upstream host.
type Proxy struct {
	fails uint32

	addr string

	// Connection caching
	expire    time.Duration
	transport *Transport

	// health checking
	probe  *up.Probe
	health HealthChecker
}

// NewProxy returns a new proxy.
func NewProxy(addr, trans string) *Proxy {
	p := &Proxy{
		addr:      addr,
		fails:     0,
		probe:     up.New(),
		transport: newTransport(addr),
	}
	p.health = NewHealthChecker(trans)
	runtime.SetFinalizer(p, (*Proxy).finalizer)
	return p
}

// SetTLSConfig sets the TLS config in the lower p.transport and in the healthchecking client.
func (p *Proxy) SetTLSConfig(cfg *tls.Config) {
	p.transport.SetTLSConfig(cfg)
	p.health.SetTLSConfig(cfg)
}

// SetExpire sets the expire duration in the lower p.transport.
func (p *Proxy) SetExpire(expire time.Duration) { p.transport.SetExpire(expire) }

// Healthcheck kicks of a round of health checks for this proxy.
func (p *Proxy) Healthcheck() {
	if p.health == nil {
		log.Warning("No healthchecker")
		return
	}

	p.probe.Do(func() error {
		return p.health.Check(p)
	})
}

// Down returns true if this proxy is down, i.e. has *more* fails than maxfails.
func (p *Proxy) Down(maxfails uint32) bool {
	if maxfails == 0 {
		return false
	}

	fails := atomic.LoadUint32(&p.fails)
	return fails > maxfails
}

// close stops the health checking goroutine.
func (p *Proxy) close()     { p.probe.Stop() }
func (p *Proxy) finalizer() { p.transport.Stop() }

// start starts the proxy's healthchecking.
func (p *Proxy) start(duration time.Duration) {
	p.probe.Start(duration)
	p.transport.Start()
}

const (
	maxTimeout = 2 * time.Second
	minTimeout = 200 * time.Millisecond
	hcInterval = 500 * time.Millisecond
)
