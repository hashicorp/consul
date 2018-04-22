package root

import (
	"os"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/mholt/caddy"
)

var log = clog.NewWithPlugin("root")

func init() {
	caddy.RegisterPlugin("root", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	config := dnsserver.GetConfig(c)

	for c.Next() {
		if !c.NextArg() {
			return plugin.Error("root", c.ArgErr())
		}
		config.Root = c.Val()
	}

	// Check if root path exists
	_, err := os.Stat(config.Root)
	if err != nil {
		if os.IsNotExist(err) {
			// Allow this, because the folder might appear later.
			// But make sure the user knows!
			log.Warningf("Root path does not exist: %s", config.Root)
		} else {
			return plugin.Error("root", c.Errf("unable to access root path '%s': %v", config.Root, err))
		}
	}

	return nil
}
