package setup

import (
	"sync"

	"github.com/miekg/coredns/middleware"
	prom "github.com/miekg/coredns/middleware/prometheus"
)

const (
	path = "/metrics"
	addr = "localhost:9135" // 9153 is occupied by bind_exporter
)

var once sync.Once

func Prometheus(c *Controller) (middleware.Middleware, error) {
	metrics, err := parsePrometheus(c)
	if err != nil {
		return nil, err
	}

	once.Do(func() {
		c.Startup = append(c.Startup, metrics.Start)
	})

	return func(next middleware.Handler) middleware.Handler {
		metrics.Next = next
		return metrics
	}, nil
}

func parsePrometheus(c *Controller) (prom.Metrics, error) {
	var (
		metrics prom.Metrics
		err     error
	)

	for c.Next() {
		if len(metrics.ZoneNames) > 0 {
			return prom.Metrics{}, c.Err("prometheus: can only have one metrics module per server")
		}
		metrics = prom.Metrics{ZoneNames: c.ServerBlockHosts}
		for i, _ := range metrics.ZoneNames {
			metrics.ZoneNames[i] = middleware.Host(metrics.ZoneNames[i]).Normalize()
		}
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			metrics.Addr = args[0]
		default:
			return prom.Metrics{}, c.ArgErr()
		}
		for c.NextBlock() {
			switch c.Val() {
			case "address":
				args = c.RemainingArgs()
				if len(args) != 1 {
					return prom.Metrics{}, c.ArgErr()
				}
				metrics.Addr = args[0]
			default:
				return prom.Metrics{}, c.Errf("prometheus: unknown item: %s", c.Val())
			}

		}
	}
	if metrics.Addr == "" {
		metrics.Addr = addr
	}
	return metrics, err
}
