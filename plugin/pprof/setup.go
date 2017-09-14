package pprof

import (
	"net"
	"sync"

	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

const defaultAddr = "localhost:6053"

func init() {
	caddy.RegisterPlugin("pprof", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	found := false
	h := &handler{addr: defaultAddr}
	for c.Next() {
		if found {
			return plugin.Error("pprof", c.Err("pprof can only be specified once"))
		}
		args := c.RemainingArgs()
		if len(args) == 1 {
			h.addr = args[0]
			_, _, e := net.SplitHostPort(h.addr)
			if e != nil {
				return e
			}
		}
		if len(args) > 1 {
			return plugin.Error("pprof", c.ArgErr())
		}
		if c.NextBlock() {
			return plugin.Error("pprof", c.ArgErr())
		}
		found = true
	}

	pprofOnce.Do(func() {
		c.OnStartup(h.Startup)
		c.OnShutdown(h.Shutdown)
	})

	return nil
}

var pprofOnce sync.Once
