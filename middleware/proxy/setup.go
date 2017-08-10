package proxy

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("proxy", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	upstreams, err := NewStaticUpstreams(&c.Dispenser)
	if err != nil {
		return middleware.Error("proxy", err)
	}

	t := dnsserver.GetConfig(c).GetHandler("trace")
	P := &Proxy{Trace: t}
	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		P.Next = next
		P.Upstreams = &upstreams
		return P
	})

	c.OnStartup(OnStartupMetrics)

	for _, u := range upstreams {
		c.OnStartup(func() error {
			return u.Exchanger().OnStartup(P)
		})
		c.OnShutdown(func() error {
			return u.Exchanger().OnShutdown(P)
		})
		// Register shutdown handlers.
		c.OnShutdown(u.Stop)
	}

	return nil
}
