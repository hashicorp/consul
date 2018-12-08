package chaos

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

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
		return plugin.Error("chaos", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Chaos{Next: next, Version: version, Authors: authors}
	})

	return nil
}

func chaosParse(c *caddy.Controller) (string, map[string]struct{}, error) {
	// Set here so we pick up AppName and AppVersion that get set in coremain's init().
	chaosVersion = caddy.AppName + "-" + caddy.AppVersion

	version := ""
	authors := make(map[string]struct{})

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return chaosVersion, nil, nil
		}
		if len(args) == 1 {
			return args[0], nil, nil
		}
		version = args[0]
		for _, a := range args[1:] {
			authors[a] = struct{}{}
		}
		return version, authors, nil
	}
	return version, authors, nil
}

var chaosVersion string
