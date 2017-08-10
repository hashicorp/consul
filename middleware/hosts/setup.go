package hosts

import (
	"log"
	"os"
	"path"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("hosts", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	h, err := hostsParse(c)
	if err != nil {
		return middleware.Error("hosts", err)
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		h.Next = next
		return h
	})

	return nil
}

func hostsParse(c *caddy.Controller) (Hosts, error) {
	var h = Hosts{
		Hostsfile: &Hostsfile{path: "/etc/hosts"},
	}
	defer h.ReadHosts()

	config := dnsserver.GetConfig(c)

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) >= 1 {
			h.path = args[0]
			args = args[1:]

			if !path.IsAbs(h.path) && config.Root != "" {
				h.path = path.Join(config.Root, h.path)
			}
			_, err := os.Stat(h.path)
			if err != nil {
				if os.IsNotExist(err) {
					log.Printf("[WARNING] File does not exist: %s", h.path)
				} else {
					return h, c.Errf("unable to access hosts file '%s': %v", h.path, err)
				}
			}
		}

		origins := make([]string, len(c.ServerBlockKeys))
		copy(origins, c.ServerBlockKeys)
		if len(args) > 0 {
			origins = args
		}

		for i := range origins {
			origins[i] = middleware.Host(origins[i]).Normalize()
		}
		h.Origins = origins

		for c.NextBlock() {
			switch c.Val() {
			case "fallthrough":
				args := c.RemainingArgs()
				if len(args) == 0 {
					h.Fallthrough = true
					continue
				}
				return h, c.ArgErr()
			}
		}
	}
	return h, nil
}
