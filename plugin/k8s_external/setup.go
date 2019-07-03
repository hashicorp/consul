package external

import (
	"strconv"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/caddyserver/caddy"
)

func init() {
	caddy.RegisterPlugin("k8s_external", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	e, err := parse(c)
	if err != nil {
		return plugin.Error("k8s_external", err)
	}

	// Do this in OnStartup, so all plugins have been initialized.
	c.OnStartup(func() error {
		m := dnsserver.GetConfig(c).Handler("kubernetes")
		if m == nil {
			return nil
		}
		if x, ok := m.(Externaler); ok {
			e.externalFunc = x.External
			e.externalAddrFunc = x.ExternalAddress
		}
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	return nil
}

func parse(c *caddy.Controller) (*External, error) {
	e := New()

	for c.Next() { // external
		zones := c.RemainingArgs()
		e.Zones = zones
		if len(zones) == 0 {
			e.Zones = make([]string, len(c.ServerBlockKeys))
			copy(e.Zones, c.ServerBlockKeys)
		}
		for i, str := range e.Zones {
			e.Zones[i] = plugin.Host(str).Normalize()
		}
		for c.NextBlock() {
			switch c.Val() {
			case "ttl":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, c.ArgErr()
				}
				t, err := strconv.Atoi(args[0])
				if err != nil {
					return nil, err
				}
				if t < 0 || t > 3600 {
					return nil, c.Errf("ttl must be in range [0, 3600]: %d", t)
				}
				e.ttl = uint32(t)
			case "apex":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, c.ArgErr()
				}
				e.apex = args[0]
			default:
				return nil, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	return e, nil
}
