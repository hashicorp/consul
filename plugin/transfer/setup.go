package transfer

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	parsepkg "github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/coredns/coredns/plugin/pkg/transport"

	"github.com/caddyserver/caddy"
)

func init() {
	caddy.RegisterPlugin("transfer", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	t, err := parse(c)

	if err != nil {
		return plugin.Error("transfer", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		t.Next = next
		return t
	})

	c.OnStartup(func() error {
		// find all plugins that implement Transferer and add them to Transferers
		plugins := dnsserver.GetConfig(c).Handlers()
		for _, pl := range plugins {
			tr, ok := pl.(Transferer)
			if !ok {
				continue
			}
			t.Transferers = append(t.Transferers, tr)
		}
		return nil
	})

	return nil
}

func parse(c *caddy.Controller) (*Transfer, error) {

	t := &Transfer{}
	for c.Next() {
		x := &xfr{}
		zones := c.RemainingArgs()

		if len(zones) != 0 {
			x.Zones = zones
			for i := 0; i < len(x.Zones); i++ {
				nzone, err := plugin.Host(x.Zones[i]).MustNormalize()
				if err != nil {
					return nil, err
				}
				x.Zones[i] = nzone
			}
		} else {
			x.Zones = make([]string, len(c.ServerBlockKeys))
			for i := 0; i < len(c.ServerBlockKeys); i++ {
				nzone, err := plugin.Host(c.ServerBlockKeys[i]).MustNormalize()
				if err != nil {
					return nil, err
				}
				x.Zones[i] = nzone
			}
		}

		for c.NextBlock() {
			switch c.Val() {
			case "to":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, c.ArgErr()
				}
				for _, host := range args {
					if host == "*" {
						x.to = append(x.to, host)
						continue
					}
					normalized, err := parsepkg.HostPort(host, transport.Port)
					if err != nil {
						return nil, err
					}
					x.to = append(x.to, normalized)
				}
			default:
				return nil, plugin.Error("transfer", c.Errf("unknown property '%s'", c.Val()))
			}
		}
		if len(x.to) == 0 {
			return nil, plugin.Error("transfer", c.Errf("'to' is required", c.Val()))
		}
		t.xfrs = append(t.xfrs, x)
	}
	return t, nil
}
