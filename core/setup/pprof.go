package setup

import (
	"sync"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pprof"
)

var pprofOnce sync.Once

// PProf returns a new instance of a pprof handler. It accepts no arguments or options.
func PProf(c *Controller) (middleware.Middleware, error) {
	found := false
	for c.Next() {
		if found {
			return nil, c.Err("pprof can only be specified once")
		}
		if len(c.RemainingArgs()) != 0 {
			return nil, c.ArgErr()
		}
		if c.NextBlock() {
			return nil, c.ArgErr()
		}
		found = true
	}
	handler := &pprof.Handler{}
	pprofOnce.Do(func() {
		c.Startup = append(c.Startup, handler.Start)
		c.Shutdown = append(c.Shutdown, handler.Shutdown)
	})
	return nil, nil
}
