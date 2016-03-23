package setup

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/loadbalance"
)

// Root sets up the root file path of the server.
func Loadbalance(c *Controller) (middleware.Middleware, error) {
	for c.Next() {
		// and choosing the correct balancer
		// TODO(miek): block and option parsing
	}
	return func(next middleware.Handler) middleware.Handler {
		return loadbalance.RoundRobin{Next: next}
	}, nil

	return nil, nil
}
