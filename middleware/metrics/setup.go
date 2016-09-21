package metrics

import (
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
		c.OnStartup(m.Startup)
		c.OnShutdown(m.Shutdown)
	})

	return nil
}

func prometheusParse(c *caddy.Controller) (Metrics, error) {
	var (
		met Metrics
		err error
	)

	for c.Next() {
		if len(met.ZoneNames) > 0 {
			return Metrics{}, c.Err("metrics: can only have one metrics module per server")
		}
		met.ZoneNames = make([]string, len(c.ServerBlockKeys))
		copy(met.ZoneNames, c.ServerBlockKeys)
		for i := range met.ZoneNames {
			met.ZoneNames[i] = middleware.Host(met.ZoneNames[i]).Normalize()
		}
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			met.Addr = args[0]
		default:
			return Metrics{}, c.ArgErr()
		}
		for c.NextBlock() {
			switch c.Val() {
			case "address":
				args = c.RemainingArgs()
				if len(args) != 1 {
					return Metrics{}, c.ArgErr()
				}
				met.Addr = args[0]
			default:
				return Metrics{}, c.Errf("metrics: unknown item: %s", c.Val())
			}

		}
	}
	if met.Addr == "" {
		met.Addr = addr
	}
	return met, err
}

var metricsOnce *sync.Once

const addr = "localhost:9153"
