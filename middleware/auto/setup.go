package auto

import (
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file"
	"github.com/miekg/coredns/middleware/metrics"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("auto", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	a, err := autoParse(c)
	if err != nil {
		return middleware.Error("auto", err)
	}

	// If we have enabled prometheus we should add newly discovered zones to it.
	met := dnsserver.GetMiddleware(c, "prometheus")
	if met != nil {
		a.metrics = met.(*metrics.Metrics)
	}

	walkChan := make(chan bool)

	c.OnStartup(func() error {
		err := a.Walk()
		if err != nil {
			return err
		}

		go func() {
			ticker := time.NewTicker(a.loader.duration)
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

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		a.Next = next
		return a
	})

	return nil
}

func autoParse(c *caddy.Controller) (Auto, error) {
	var a = Auto{
		loader: loader{template: "${1}", re: regexp.MustCompile(`db\.(.*)`), duration: time.Duration(60 * time.Second)},
		Zones:  &Zones{},
	}

	config := dnsserver.GetConfig(c)

	for c.Next() {
		if c.Val() == "auto" {
			// auto [ZONES...]
			a.Zones.origins = make([]string, len(c.ServerBlockKeys))
			copy(a.Zones.origins, c.ServerBlockKeys)

			args := c.RemainingArgs()
			if len(args) > 0 {
				a.Zones.origins = args
			}
			for i := range a.Zones.origins {
				a.Zones.origins[i] = middleware.Host(a.Zones.origins[i]).Normalize()
			}

			for c.NextBlock() {
				switch c.Val() {
				case "directory": // directory DIR [REGEXP [TEMPLATE] [DURATION]]
					if !c.NextArg() {
						return a, c.ArgErr()
					}
					a.loader.directory = c.Val()
					if !path.IsAbs(a.loader.directory) && config.Root != "" {
						a.loader.directory = path.Join(config.Root, a.loader.directory)
					}
					_, err := os.Stat(a.loader.directory)
					if err != nil {
						if os.IsNotExist(err) {
							log.Printf("[WARNING] Directory does not exist: %s", a.loader.directory)
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

					// duration
					if c.NextArg() {
						i, err := strconv.Atoi(c.Val())
						if err != nil {
							return a, err
						}
						if i < 1 {
							i = 1
						}
						a.loader.duration = time.Duration(i) * time.Second
					}

				case "no_reload":
					a.loader.noReload = true

				default:
					t, _, e := file.TransferParse(c, false)
					if e != nil {
						return a, e
					}
					if t != nil {
						a.loader.transferTo = append(a.loader.transferTo, t...)
					}
				}
			}

		}
	}
	return a, nil
}
