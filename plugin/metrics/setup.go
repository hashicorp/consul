package metrics

import (
	"net"
	"runtime"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics/vars"
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
	m, err := parse(c)
	if err != nil {
		return plugin.Error("prometheus", err)
	}

	c.OnStartup(func() error { m.Reg = uniqAddr.Set(m.Addr, m.OnStartup, m).(*Metrics).Reg; return nil })
	c.OnRestartFailed(func() error { m.Reg = uniqAddr.Set(m.Addr, m.OnStartup, m).(*Metrics).Reg; return nil })

	c.OnStartup(func() error { return uniqAddr.ForEach() })
	c.OnRestartFailed(func() error { return uniqAddr.ForEach() })

	c.OnStartup(func() error {
		conf := dnsserver.GetConfig(c)
		for _, h := range conf.ListenHosts {
			addrstr := conf.Transport + "://" + net.JoinHostPort(h, conf.Port)
			for _, p := range conf.Handlers() {
				vars.PluginEnabled.WithLabelValues(addrstr, conf.Zone, p.Name()).Set(1)
			}
		}
		return nil
	})
	c.OnRestartFailed(func() error {
		conf := dnsserver.GetConfig(c)
		for _, h := range conf.ListenHosts {
			addrstr := conf.Transport + "://" + net.JoinHostPort(h, conf.Port)
			for _, p := range conf.Handlers() {
				vars.PluginEnabled.WithLabelValues(addrstr, conf.Zone, p.Name()).Set(1)
			}
		}
		return nil
	})

	c.OnRestart(m.OnRestart)
	c.OnRestart(func() error { vars.PluginEnabled.Reset(); return nil })
	c.OnFinalShutdown(m.OnFinalShutdown)

	// Initialize metrics.
	buildInfo.WithLabelValues(coremain.CoreVersion, coremain.GitCommit, runtime.Version()).Set(1)

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		m.Next = next
		return m
	})

	return nil
}

func parse(c *caddy.Controller) (*Metrics, error) {
	var met = New(defaultAddr)

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
