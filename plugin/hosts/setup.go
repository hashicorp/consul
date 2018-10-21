package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/mholt/caddy"
)

var log = clog.NewWithPlugin("hosts")

func init() {
	caddy.RegisterPlugin("hosts", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	h, err := hostsParse(c)
	if err != nil {
		return plugin.Error("hosts", err)
	}

	parseChan := make(chan bool)

	c.OnStartup(func() error {
		h.readHosts()

		go func() {
			ticker := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-parseChan:
					return
				case <-ticker.C:
					h.readHosts()
				}
			}
		}()
		return nil
	})

	c.OnShutdown(func() error {
		close(parseChan)
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})

	return nil
}

func hostsParse(c *caddy.Controller) (Hosts, error) {
	var h = Hosts{
		Hostsfile: &Hostsfile{
			path: "/etc/hosts",
			hmap: newHostsMap(),
		},
	}

	config := dnsserver.GetConfig(c)

	inline := []string{}
	i := 0
	for c.Next() {
		if i > 0 {
			return h, plugin.ErrOnce
		}
		i++

		args := c.RemainingArgs()
		if len(args) >= 1 {
			h.path = args[0]
			args = args[1:]

			if !filepath.IsAbs(h.path) && config.Root != "" {
				h.path = filepath.Join(config.Root, h.path)
			}
			s, err := os.Stat(h.path)
			if err != nil {
				if os.IsNotExist(err) {
					log.Warningf("File does not exist: %s", h.path)
				} else {
					return h, c.Errf("unable to access hosts file '%s': %v", h.path, err)
				}
			}
			if s != nil && s.IsDir() {
				log.Warningf("Hosts file %q is a directory", h.path)
			}
		}

		origins := make([]string, len(c.ServerBlockKeys))
		copy(origins, c.ServerBlockKeys)
		if len(args) > 0 {
			origins = args
		}

		for i := range origins {
			origins[i] = plugin.Host(origins[i]).Normalize()
		}
		h.Origins = origins

		for c.NextBlock() {
			switch c.Val() {
			case "fallthrough":
				h.Fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				if len(h.Fall.Zones) == 0 {
					line := strings.Join(append([]string{c.Val()}, c.RemainingArgs()...), " ")
					inline = append(inline, line)
					continue
				}
				return h, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}

	h.initInline(inline)

	return h, nil
}
