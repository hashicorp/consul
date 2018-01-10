package health

import (
	"net"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("health", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	addr, err := healthParse(c)
	if err != nil {
		return plugin.Error("health", err)
	}

	h := &health{Addr: addr, stop: make(chan bool)}

	c.OnStartup(func() error {
		plugins := dnsserver.GetConfig(c).Handlers()
		for _, p := range plugins {
			if x, ok := p.(Healther); ok {
				h.h = append(h.h, x)
			}
		}
		return nil
	})

	c.OnStartup(func() error {
		// Poll all middleware every second.
		h.poll()
		go func() {
			for {
				<-time.After(1 * time.Second)
				h.poll()
			}
		}()
		return nil
	})

	c.OnStartup(func() error {
		onceMetric.Do(func() {
			m := dnsserver.GetConfig(c).Handler("prometheus")
			if m == nil {
				return
			}
			if x, ok := m.(*metrics.Metrics); ok {
				x.MustRegister(HealthDuration)
			}
		})
		return nil
	})

	c.OnStartup(h.OnStartup)
	c.OnFinalShutdown(h.OnShutdown)

	// Don't do AddPlugin, as health is not *really* a plugin just a separate webserver running.
	return nil
}

func healthParse(c *caddy.Controller) (string, error) {
	addr := ""
	for c.Next() {
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			addr = args[0]
			if _, _, e := net.SplitHostPort(addr); e != nil {
				return "", e
			}
		default:
			return "", c.ArgErr()
		}
	}
	return addr, nil
}
