package metrics

import (
	"net"
	"runtime"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/uniq"

	"github.com/mholt/caddy"
)

var (
	log      = clog.NewWithPlugin("prometheus")
	uniqAddr = uniq.New()
)

func init() {
	caddy.RegisterPlugin("prometheus", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	m, err := prometheusParse(c)
	if err != nil {
		return plugin.Error("prometheus", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		m.Next = next
		return m
	})

	c.OncePerServerBlock(func() error {
		c.OnStartup(func() error {
			return uniqAddr.ForEach()
		})
		return nil
	})

	c.OnRestart(m.OnRestart)
	c.OnFinalShutdown(m.OnFinalShutdown)

	// Initialize metrics.
	buildInfo.WithLabelValues(coremain.CoreVersion, coremain.GitCommit, runtime.Version()).Set(1)

	return nil
}

func prometheusParse(c *caddy.Controller) (*Metrics, error) {
	var met = New(defaultAddr)

	defer func() {
		uniqAddr.Set(met.Addr, met.OnStartup)
	}()

	i := 0
	for c.Next() {
		if i > 0 {
			return nil, plugin.ErrOnce
		}
		i++

		for _, z := range c.ServerBlockKeys {
			met.AddZone(plugin.Host(z).Normalize())
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
	}
	return met, nil
}

// defaultAddr is the address the where the metrics are exported by default.
const defaultAddr = "localhost:9153"
