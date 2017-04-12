package metrics

import (
	"net"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("prometheus", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})

	uniqAddr = addrs{a: make(map[string]int)}
}

func setup(c *caddy.Controller) error {
	m, err := prometheusParse(c)
	if err != nil {
		return middleware.Error("prometheus", err)
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		m.Next = next
		return m
	})

	for a, v := range uniqAddr.a {
		if v == todo {
			// During restarts we will keep this handler running, BUG.
			c.OncePerServerBlock(m.OnStartup)
		}
		uniqAddr.a[a] = done
	}
	c.OnFinalShutdown(m.OnShutdown)

	return nil
}

func prometheusParse(c *caddy.Controller) (*Metrics, error) {
	var (
		met = &Metrics{Addr: addr, zoneMap: make(map[string]bool)}
		err error
	)

	defer func() {
		uniqAddr.SetAddress(met.Addr)
	}()

	for c.Next() {
		if len(met.ZoneNames()) > 0 {
			return met, c.Err("can only have one metrics module per server")
		}

		for _, z := range c.ServerBlockKeys {
			met.AddZone(middleware.Host(z).Normalize())
		}
		args := c.RemainingArgs()

		switch len(args) {
		case 0:
		case 1:
			met.Addr = args[0]
			_, _, e := net.SplitHostPort(met.Addr)
			if e != nil {
				return met, e
			}
		default:
			return met, c.ArgErr()
		}
		for c.NextBlock() {
			switch c.Val() {
			case "address":
				args = c.RemainingArgs()
				if len(args) != 1 {
					return met, c.ArgErr()
				}
				met.Addr = args[0]
				// expecting something that resembles a host-port
				_, _, e := net.SplitHostPort(met.Addr)
				if e != nil {
					return met, e
				}
			default:
				return met, c.Errf("unknown item: %s", c.Val())
			}

		}
	}
	return met, err
}

var uniqAddr addrs

// Keep track on which addrs we listen, so we only start one listener.
type addrs struct {
	a map[string]int
}

func (a *addrs) SetAddress(addr string) {
	// If already there and set to done, we've already started this listener.
	if a.a[addr] == done {
		return
	}
	a.a[addr] = todo
}

// Addr is the address the where the metrics are exported by default.
const addr = "localhost:9153"

const (
	todo = 1
	done = 2
)
