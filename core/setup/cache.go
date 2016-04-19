package setup

import (
	"strconv"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/cache"
)

// Cache sets up the root file path of the server.
func Cache(c *Controller) (middleware.Middleware, error) {
	ttl, zones, err := cacheParse(c)
	if err != nil {
		return nil, err
	}
	return func(next middleware.Handler) middleware.Handler {
		return cache.NewCache(ttl, zones, next)
	}, nil
}

func cacheParse(c *Controller) (int, []string, error) {
	var (
		err error
		ttl int
	)

	for c.Next() {
		if c.Val() == "cache" {
			// cache [ttl] [zones..]

			origins := []string{c.ServerBlockHosts[c.ServerBlockHostIndex]}
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
						origins = []string{c.ServerBlockHosts[c.ServerBlockHostIndex]}
					}
				}
			}

			for i, _ := range origins {
				origins[i] = middleware.Host(origins[i]).Normalize()
			}
			return ttl, origins, nil
		}
	}
	return 0, nil, nil
}
