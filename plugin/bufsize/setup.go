package bufsize

import (
	"strconv"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/caddyserver/caddy"
)

func init() { plugin.Register("bufsize", setup) }

func setup(c *caddy.Controller) error {
	bufsize, err := parse(c)
	if err != nil {
		return plugin.Error("bufsize", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Bufsize{Next: next, Size: bufsize}
	})

	return nil
}

func parse(c *caddy.Controller) (int, error) {
	const defaultBufSize = 512
	for c.Next() {
		args := c.RemainingArgs()
		switch len(args) {
		case 0:
			// Nothing specified; use 512 as default
			return defaultBufSize, nil
		case 1:
			// Specified value is needed to verify
			bufsize, err := strconv.Atoi(args[0])
			if err != nil {
				return -1, plugin.Error("bufsize", c.ArgErr())
			}
			// Follows RFC 6891
			if bufsize < 512 || bufsize > 4096 {
				return -1, plugin.Error("bufsize", c.ArgErr())
			}
			return bufsize, nil
		default:
			// Only 1 argument is acceptable
			return -1, plugin.Error("bufsize", c.ArgErr())
		}
	}
	return -1, plugin.Error("bufsize", c.ArgErr())
}
