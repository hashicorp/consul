package whoami

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("whoami", caddy.Plugin{
		ServerType: "dns",
		Action:     setupWhoami,
	})
}

func setupWhoami(c *caddy.Controller) error {
	c.Next() // 'whoami'
	if c.NextArg() {
		return middleware.Error("whoami", c.ArgErr())
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return Whoami{}
	})

	return nil
}
