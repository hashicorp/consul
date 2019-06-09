package ready

import (
	"net"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("ready", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	addr, err := parse(c)
	if err != nil {
		return plugin.Error("ready", err)
	}
	rd := &ready{Addr: addr}

	uniqAddr.Set(addr, rd.onStartup)
	c.OnStartup(func() error { uniqAddr.Set(addr, rd.onStartup); return nil })
	c.OnRestartFailed(func() error { uniqAddr.Set(addr, rd.onStartup); return nil })

	c.OnStartup(func() error { return uniqAddr.ForEach() })
	c.OnRestartFailed(func() error { return uniqAddr.ForEach() })

	c.OnStartup(func() error {
		for _, p := range dnsserver.GetConfig(c).Handlers() {
			if r, ok := p.(Readiness); ok {
				plugins.Append(r, p.Name())
			}
		}
		return nil
	})
	c.OnRestartFailed(func() error {
		for _, p := range dnsserver.GetConfig(c).Handlers() {
			if r, ok := p.(Readiness); ok {
				plugins.Append(r, p.Name())
			}
		}
		return nil
	})

	c.OnRestart(rd.onFinalShutdown)
	c.OnFinalShutdown(rd.onFinalShutdown)

	return nil
}

func parse(c *caddy.Controller) (string, error) {
	addr := ":8181"
	i := 0
	for c.Next() {
		if i > 0 {
			return "", plugin.ErrOnce
		}
		i++
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
