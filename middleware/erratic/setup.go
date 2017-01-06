package erratic

import (
	"fmt"
	"strconv"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("erratic", caddy.Plugin{
		ServerType: "dns",
		Action:     setupErratic,
	})
}

func setupErratic(c *caddy.Controller) error {
	e, err := parseErratic(c)
	if err != nil {
		return middleware.Error("erratic", err)
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return e
	})

	return nil
}

func parseErratic(c *caddy.Controller) (*Erratic, error) {
	e := &Erratic{amount: 2}
	for c.Next() { // 'erratic'
		for c.NextBlock() {
			switch c.Val() {
			case "drop":
				args := c.RemainingArgs()
				if len(args) > 1 {
					return nil, c.ArgErr()
				}

				if len(args) == 0 {
					return nil, nil
				}
				amount, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return nil, err
				}
				if amount < 0 {
					return nil, fmt.Errorf("illegal amount value given %q", args[0])
				}
				e.amount = uint64(amount)
			}
		}
	}
	return e, nil
}
