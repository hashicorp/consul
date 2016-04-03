package setup

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file"
	"github.com/miekg/coredns/middleware/secondary"
)

// Secondary sets up the secondary middleware.
func Secondary(c *Controller) (middleware.Middleware, error) {
	zones, err := secondaryParse(c)
	if err != nil {
		return nil, err
	}

	// Setup retrieve the zone.
	for _, n := range zones.Names {
		if len(zones.Z[n].TransferFrom) > 0 {
			c.Startup = append(c.Startup, func() error {
				err := zones.Z[n].TransferIn()
				return err
			})
		}
	}

	return func(next middleware.Handler) middleware.Handler {
		return secondary.Secondary{file.File{Next: next, Zones: zones}}
	}, nil

}

func secondaryParse(c *Controller) (file.Zones, error) {
	z := make(map[string]*file.Zone)
	names := []string{}
	for c.Next() {
		if c.Val() == "secondary" {
			// secondary [origin]
			origin := c.ServerBlockHosts[c.ServerBlockHostIndex]
			if c.NextArg() {
				origin = c.Val()
			}
			// TODO(miek): we should allow more. Issue #54.
			origin = middleware.Host(origin).Normalize()

			z[origin] = file.NewZone(origin)
			names = append(names, origin)

			for c.NextBlock() {
				t, f, e := parseTransfer(c)
				if e != nil {
					return file.Zones{}, e
				}
				z[origin].TransferTo = append(z[origin].TransferTo, t)
				z[origin].TransferFrom = append(z[origin].TransferFrom, f)
			}
		}
	}
	return file.Zones{Z: z, Names: names}, nil
}
