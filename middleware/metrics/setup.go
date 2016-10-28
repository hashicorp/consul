package metrics

import (
	"net"
	"sync"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("prometheus", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	m, err := prometheusParse(c)
	if err != nil {
		return middleware.Error("prometheus", err)
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		m.Next = next
		return m
	})

	metricsOnce.Do(func() {
		c.OnStartup(m.OnStartup)
		c.OnShutdown(m.OnShutdown)
	})

	return nil
}

func prometheusParse(c *caddy.Controller) (*Metrics, error) {
	var (
		met = &Metrics{Addr: addr, zoneMap: make(map[string]bool)}
		err error
	)

	for c.Next() {
		if len(met.ZoneNames()) > 0 {
			return met, c.Err("can only have one metrics module per server")
		}

		for _, z := range c.ServerBlockKeys {
			met.AddZone(middleware.Host(z).Normalize())
		}
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			met.Addr = args[0]
			_, _, e := net.SplitHostPort(met.Addr)
			if e != nil {
				return met, e
			}
		default:
			return met, c.ArgErr()
		}
		for c.NextBlock() {
			switch c.Val() {
			case "address":
				args = c.RemainingArgs()
				if len(args) != 1 {
					return met, c.ArgErr()
				}
				met.Addr = args[0]
				// expecting something that resembles a host-port
				_, _, e := net.SplitHostPort(met.Addr)
				if e != nil {
					return met, e
				}
			default:
				return met, c.Errf("unknown item: %s", c.Val())
			}

		}
	}
	return met, err
}

var metricsOnce sync.Once

// Addr is the address the where the metrics are exported by default.
const addr = "localhost:9153"
