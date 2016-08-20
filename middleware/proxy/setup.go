package proxy

import (
	"github.com/miekg/coredns/core/dnsserver"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("proxy", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	upstreams, err := NewStaticUpstreams(c.Dispenser)
	if err != nil {
		return err
	}
	dnsserver.GetConfig(c).AddMiddleware(func(next dnsserver.Handler) dnsserver.Handler {
		return Proxy{Next: next, Client: Clients(), Upstreams: upstreams}
	})

	return nil
}
