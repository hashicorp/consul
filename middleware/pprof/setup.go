package pprof

import (
	"sync"

	"github.com/mholt/caddy"
	"github.com/miekg/coredns/middleware"
)

func init() {
	caddy.RegisterPlugin("pprof", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	found := false
	for c.Next() {
		if found {
			return middleware.Error("pprof", c.Err("pprof can only be specified once"))
		}
		if len(c.RemainingArgs()) != 0 {
			return middleware.Error("pprof", c.ArgErr())
		}
		if c.NextBlock() {
			return middleware.Error("pprof", c.ArgErr())
		}
		found = true
	}

	h := &handler{}
	pprofOnce.Do(func() {
		c.OnStartup(h.Startup)
		c.OnShutdown(h.Shutdown)
	})

	return nil
}

var pprofOnce sync.Once
