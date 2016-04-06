package setup

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/health"
)

func Health(c *Controller) (middleware.Middleware, error) {
	addr, err := parseHealth(c)
	if err != nil {
		return nil, err
	}

	h := health.Health{Addr: addr}
	c.Startup = append(c.Startup, h.ListenAndServe)
	return nil, nil
}

func parseHealth(c *Controller) (string, error) {
	addr := ""
	for c.Next() {
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			addr = args[0]
		default:
			return "", c.ArgErr()
		}
	}
	return addr, nil
}
