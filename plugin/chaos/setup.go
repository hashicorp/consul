//go:generate go run owners_generate.go

package chaos

import (
	"sort"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/caddyserver/caddy"
)

func init() { plugin.Register("chaos", setup) }

func setup(c *caddy.Controller) error {
	version, authors, err := parse(c)
	if err != nil {
		return plugin.Error("chaos", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Chaos{Next: next, Version: version, Authors: authors}
	})

	return nil
}

func parse(c *caddy.Controller) (string, []string, error) {
	// Set here so we pick up AppName and AppVersion that get set in coremain's init().
	chaosVersion = caddy.AppName + "-" + caddy.AppVersion
	version := ""

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return trim(chaosVersion), Owners, nil
		}
		if len(args) == 1 {
			return trim(args[0]), Owners, nil
		}

		version = args[0]
		authors := make(map[string]struct{})
		for _, a := range args[1:] {
			authors[a] = struct{}{}
		}
		list := []string{}
		for k := range authors {
			k = trim(k) // limit size to 255 chars
			list = append(list, k)
		}
		sort.Strings(list)
		return version, list, nil
	}

	return version, Owners, nil
}

func trim(s string) string {
	if len(s) < 256 {
		return s
	}
	return s[:255]
}

var chaosVersion string
