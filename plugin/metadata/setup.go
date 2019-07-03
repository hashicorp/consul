package metadata

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/caddyserver/caddy"
)

func init() {
	caddy.RegisterPlugin("metadata", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	m, err := metadataParse(c)
	if err != nil {
		return err
	}
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		m.Next = next
		return m
	})

	c.OnStartup(func() error {
		plugins := dnsserver.GetConfig(c).Handlers()
		for _, p := range plugins {
			if met, ok := p.(Provider); ok {
				m.Providers = append(m.Providers, met)
			}
		}
		return nil
	})

	return nil
}

func metadataParse(c *caddy.Controller) (*Metadata, error) {
	m := &Metadata{}
	c.Next()
	zones := c.RemainingArgs()

	if len(zones) != 0 {
		m.Zones = zones
		for i := 0; i < len(m.Zones); i++ {
			m.Zones[i] = plugin.Host(m.Zones[i]).Normalize()
		}
	} else {
		m.Zones = make([]string, len(c.ServerBlockKeys))
		for i := 0; i < len(c.ServerBlockKeys); i++ {
			m.Zones[i] = plugin.Host(c.ServerBlockKeys[i]).Normalize()
		}
	}

	if c.NextBlock() || c.Next() {
		return nil, plugin.Error("metadata", c.ArgErr())
	}
	return m, nil
}
