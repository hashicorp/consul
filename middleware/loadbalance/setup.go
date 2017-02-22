package loadbalance

import (
	"github.com/mholt/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"
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

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return RoundRobin{Next: next}
	})

	return nil
}
