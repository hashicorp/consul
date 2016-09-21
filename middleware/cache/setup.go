package cache

import (
	"strconv"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

// Cache sets up the root file path of the server.
func setup(c *caddy.Controller) error {
	ttl, zones, err := cacheParse(c)
	if err != nil {
		return middleware.Error("cache", err)
	}
	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return NewCache(ttl, zones, next)
	})

	return nil
}

func cacheParse(c *caddy.Controller) (int, []string, error) {
	var (
		err     error
		ttl     int
		origins []string
	)

	for c.Next() {
		if c.Val() == "cache" {
			// cache [ttl] [zones..]
			origins = make([]string, len(c.ServerBlockKeys))
			copy(origins, c.ServerBlockKeys)
			args := c.RemainingArgs()
			if len(args) > 0 {
				origins = args
				// first args may be just a number, then it is the ttl, if not it is a zone
				t := origins[0]
				ttl, err = strconv.Atoi(t)
				if err == nil {
					origins = origins[1:]
					if len(origins) == 0 {
						// There was *only* the ttl, revert back to server block
						copy(origins, c.ServerBlockKeys)
					}
				}
			}

			for i := range origins {
				origins[i] = middleware.Host(origins[i]).Normalize()
			}
			return ttl, origins, nil
		}
	}
	return 0, nil, nil
}
