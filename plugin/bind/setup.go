package bind

import (
	"fmt"
	"net"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

func setupBind(c *caddy.Controller) error {
	config := dnsserver.GetConfig(c)
	for c.Next() {
		if !c.Args(&config.ListenHost) {
			return plugin.Error("bind", c.ArgErr())
		}
	}
	if net.ParseIP(config.ListenHost) == nil {
		return plugin.Error("bind", fmt.Errorf("not a valid IP address: %s", config.ListenHost))
	}
	return nil
}
