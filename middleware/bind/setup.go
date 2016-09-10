package bind

import (
	"fmt"
	"net"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

func setupBind(c *caddy.Controller) error {
	config := dnsserver.GetConfig(c)
	for c.Next() {
		if !c.Args(&config.ListenHost) {
			return middleware.Error("bind", c.ArgErr())
		}
	}
	if net.ParseIP(config.ListenHost) == nil {
		return middleware.Error("bind", fmt.Errorf("not a valid IP address: %s", config.ListenHost))
	}
	return nil
}
