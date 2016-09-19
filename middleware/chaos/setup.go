package chaos

import (
	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("chaos", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	version, authors, err := chaosParse(c)
	if err != nil {
		return middleware.Error("chaos", err)
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return Chaos{Next: next, Version: version, Authors: authors}
	})

	return nil
}

func chaosParse(c *caddy.Controller) (string, map[string]bool, error) {
	version := ""
	authors := make(map[string]bool)

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return defaultVersion, nil, nil
		}
		if len(args) == 1 {
			return args[0], nil, nil
		}
		version = args[0]
		for _, a := range args[1:] {
			authors[a] = true
		}
		return version, authors, nil
	}
	return version, authors, nil
}

var defaultVersion = caddy.AppName + "-" + caddy.AppVersion
