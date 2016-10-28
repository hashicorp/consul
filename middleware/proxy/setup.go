package proxy

import (
	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

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
	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return Proxy{Next: next, Client: newClient(), Upstreams: upstreams}
	})

	c.OnStartup(OnStartup)

	return nil
}
