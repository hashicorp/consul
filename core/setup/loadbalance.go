package setup

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/loadbalance"
)

// Loadbalance sets up the root file path of the server.
func Loadbalance(c *Controller) (middleware.Middleware, error) {
	for c.Next() {
		// TODO(miek): block and option parsing
	}
	return func(next middleware.Handler) middleware.Handler {
		return loadbalance.RoundRobin{Next: next}
	}, nil
}
