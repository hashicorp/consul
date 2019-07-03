package erratic

import (
	"fmt"
	"strconv"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/caddyserver/caddy"
)

func init() {
	caddy.RegisterPlugin("erratic", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	e, err := parseErratic(c)
	if err != nil {
		return plugin.Error("erratic", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return e
	})

	return nil
}

func parseErratic(c *caddy.Controller) (*Erratic, error) {
	e := &Erratic{drop: 2}
	drop := false // true if we've seen the drop keyword

	for c.Next() { // 'erratic'
		for c.NextBlock() {
			switch c.Val() {
			case "drop":
				args := c.RemainingArgs()
				if len(args) > 1 {
					return nil, c.ArgErr()
				}

				if len(args) == 0 {
					continue
				}

				amount, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return nil, err
				}
				if amount < 0 {
					return nil, fmt.Errorf("illegal amount value given %q", args[0])
				}
				e.drop = uint64(amount)
				drop = true
			case "delay":
				args := c.RemainingArgs()
				if len(args) > 2 {
					return nil, c.ArgErr()
				}

				// Defaults.
				e.delay = 2
				e.duration = 100 * time.Millisecond
				if len(args) == 0 {
					continue
				}

				amount, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return nil, err
				}
				if amount < 0 {
					return nil, fmt.Errorf("illegal amount value given %q", args[0])
				}
				e.delay = uint64(amount)

				if len(args) > 1 {
					duration, err := time.ParseDuration(args[1])
					if err != nil {
						return nil, err
					}
					e.duration = duration
				}
			case "truncate":
				args := c.RemainingArgs()
				if len(args) > 1 {
					return nil, c.ArgErr()
				}

				if len(args) == 0 {
					continue
				}

				amount, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return nil, err
				}
				if amount < 0 {
					return nil, fmt.Errorf("illegal amount value given %q", args[0])
				}
				e.truncate = uint64(amount)
			case "large":
				e.large = true
			default:
				return nil, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	if (e.delay > 0 || e.truncate > 0) && !drop { // delay is set, but we've haven't seen a drop keyword, remove default drop stuff
		e.drop = 0
	}

	return e, nil
}
