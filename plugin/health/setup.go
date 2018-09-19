package health

import (
	"fmt"
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
	addr, lame, err := healthParse(c)
	if err != nil {
		return plugin.Error("health", err)
	}

	h := newHealth(addr)
	h.lameduck = lame

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
				select {
				case <-time.After(1 * time.Second):
					h.poll()
				case <-h.pollstop:
					return
				}
			}
		}()
		return nil
	})

	c.OnStartup(func() error {
		metrics.MustRegister(c, HealthDuration)
		return nil
	})

	c.OnStartup(h.OnStartup)
	c.OnRestart(h.OnRestart)
	c.OnFinalShutdown(h.OnFinalShutdown)

	// Don't do AddPlugin, as health is not *really* a plugin just a separate webserver running.
	return nil
}

func healthParse(c *caddy.Controller) (string, time.Duration, error) {
	addr := ""
	dur := time.Duration(0)
	for c.Next() {
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			addr = args[0]
			if _, _, e := net.SplitHostPort(addr); e != nil {
				return "", 0, e
			}
		default:
			return "", 0, c.ArgErr()
		}

		for c.NextBlock() {
			switch c.Val() {
			case "lameduck":
				args := c.RemainingArgs()
				if len(args) != 1 {
					return "", 0, c.ArgErr()
				}
				l, err := time.ParseDuration(args[0])
				if err != nil {
					return "", 0, fmt.Errorf("unable to parse lameduck duration value: '%v' : %v", args[0], err)
				}
				dur = l
			default:
				return "", 0, c.ArgErr()
			}
		}
	}
	return addr, dur, nil
}
