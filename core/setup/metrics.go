package setup

import (
	"sync"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/metrics"
)

const addr = "localhost:9153"

var metricsOnce sync.Once

func Prometheus(c *Controller) (middleware.Middleware, error) {
	met, err := parsePrometheus(c)
	if err != nil {
		return nil, err
	}

	metricsOnce.Do(func() {
		c.Startup = append(c.Startup, met.Start)
	})

	return func(next middleware.Handler) middleware.Handler {
		met.Next = next
		return met
	}, nil
}

func parsePrometheus(c *Controller) (metrics.Metrics, error) {
	var (
		met metrics.Metrics
		err error
	)

	for c.Next() {
		if len(met.ZoneNames) > 0 {
			return metrics.Metrics{}, c.Err("metrics: can only have one metrics module per server")
		}
		met = metrics.Metrics{ZoneNames: c.ServerBlockHosts}
		for i, _ := range met.ZoneNames {
			met.ZoneNames[i] = middleware.Host(met.ZoneNames[i]).Normalize()
		}
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			met.Addr = args[0]
		default:
			return metrics.Metrics{}, c.ArgErr()
		}
		for c.NextBlock() {
			switch c.Val() {
			case "address":
				args = c.RemainingArgs()
				if len(args) != 1 {
					return metrics.Metrics{}, c.ArgErr()
				}
				met.Addr = args[0]
			default:
				return metrics.Metrics{}, c.Errf("metrics: unknown item: %s", c.Val())
			}

		}
	}
	if met.Addr == "" {
		met.Addr = addr
	}
	return met, err
}
