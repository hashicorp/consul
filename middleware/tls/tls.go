package tls

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/tls"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("tls", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	config := dnsserver.GetConfig(c)

	if config.TLSConfig != nil {
		return middleware.Error("tls", c.Errf("TLS already configured for this server instance"))
	}

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) != 3 {
			return middleware.Error("tls", c.ArgErr())
		}
		tls, err := tls.NewTLSConfig(args[0], args[1], args[2])
		if err != nil {
			return middleware.Error("tls", c.ArgErr())
		}
		config.TLSConfig = tls
	}
	return nil
}
