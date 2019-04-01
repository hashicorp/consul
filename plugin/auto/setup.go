package auto

import (
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/mholt/caddy"
)

var log = clog.NewWithPlugin("auto")

func init() {
	caddy.RegisterPlugin("auto", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	a, err := autoParse(c)
	if err != nil {
		return plugin.Error("auto", err)
	}

	c.OnStartup(func() error {
		m := dnsserver.GetConfig(c).Handler("prometheus")
		if m == nil {
			return nil
		}
		(&a).metrics = m.(*metrics.Metrics)
		return nil
	})

	walkChan := make(chan bool)

	c.OnStartup(func() error {
		err := a.Walk()
		if err != nil {
			return err
		}

		go func() {
			ticker := time.NewTicker(a.loader.ReloadInterval)
			for {
				select {
				case <-walkChan:
					return
				case <-ticker.C:
					a.Walk()
				}
			}
		}()
		return nil
	})

	c.OnShutdown(func() error {
		close(walkChan)
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		a.Next = next
		return a
	})

	return nil
}

func autoParse(c *caddy.Controller) (Auto, error) {
	nilInterval := -1 * time.Second
	var a = Auto{
		loader: loader{
			template:       "${1}",
			re:             regexp.MustCompile(`db\.(.*)`),
			ReloadInterval: nilInterval,
		},
		Zones: &Zones{},
	}

	config := dnsserver.GetConfig(c)

	for c.Next() {
		// auto [ZONES...]
		a.Zones.origins = make([]string, len(c.ServerBlockKeys))
		copy(a.Zones.origins, c.ServerBlockKeys)

		args := c.RemainingArgs()
		if len(args) > 0 {
			a.Zones.origins = args
		}
		for i := range a.Zones.origins {
			a.Zones.origins[i] = plugin.Host(a.Zones.origins[i]).Normalize()
		}

		for c.NextBlock() {
			switch c.Val() {
			case "directory": // directory DIR [REGEXP TEMPLATE]
				if !c.NextArg() {
					return a, c.ArgErr()
				}
				a.loader.directory = c.Val()
				if !filepath.IsAbs(a.loader.directory) && config.Root != "" {
					a.loader.directory = filepath.Join(config.Root, a.loader.directory)
				}
				_, err := os.Stat(a.loader.directory)
				if err != nil {
					if os.IsNotExist(err) {
						log.Warningf("Directory does not exist: %s", a.loader.directory)
					} else {
						return a, c.Errf("Unable to access root path '%s': %v", a.loader.directory, err)
					}
				}

				// regexp
				if c.NextArg() {
					a.loader.re, err = regexp.Compile(c.Val())
					if err != nil {
						return a, err
					}
					if a.loader.re.NumSubexp() == 0 {
						return a, c.Errf("Need at least one sub expression")
					}
				}

				// template
				if c.NextArg() {
					a.loader.template = rewriteToExpand(c.Val())
				}

				if c.NextArg() {
					return Auto{}, c.ArgErr()
				}

			case "reload":
				d, err := time.ParseDuration(c.RemainingArgs()[0])
				if err != nil {
					return a, plugin.Error("file", err)
				}
				a.loader.ReloadInterval = d

			case "upstream":
				c.RemainingArgs() // eat remaining args
				a.loader.upstream = upstream.New()

			case "transfer":
				t, _, e := parse.Transfer(c, false)
				if e != nil {
					return a, e
				}
				if t != nil {
					a.loader.transferTo = append(a.loader.transferTo, t...)
				}

			default:
				return Auto{}, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}

	if a.loader.ReloadInterval == nilInterval {
		a.loader.ReloadInterval = 60 * time.Second
	}

	return a, nil
}
