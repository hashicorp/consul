package loadbalance

import (
	"github.com/mholt/caddy"
	"github.com/miekg/coredns/core/dnsserver"
)

func init() {
	caddy.RegisterPlugin("loadbalance", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	for c.Next() {
		// TODO(miek): block and option parsing
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next dnsserver.Handler) dnsserver.Handler {
		return RoundRobin{Next: next}
	})

	return nil
}
