package debug

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("debug", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	config := dnsserver.GetConfig(c)

	for c.Next() {
		if c.NextArg() {
			return plugin.Error("debug", c.ArgErr())
		}
		config.Debug = true
	}

	return nil
}
