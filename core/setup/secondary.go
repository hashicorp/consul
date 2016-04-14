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

	// Add startup functions to retrieve the zone and keep it up to date.
	for _, n := range zones.Names {
		if len(zones.Z[n].TransferFrom) > 0 {
			c.Startup = append(c.Startup, func() error {
				zones.Z[n].StartupOnce.Do(func() {
					zones.Z[n].TransferIn()
					go func() {
						zones.Z[n].Update()
					}()
				})
				return nil
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
			origins := []string{c.ServerBlockHosts[c.ServerBlockHostIndex]}
			args := c.RemainingArgs()
			if len(args) > 0 {
				origins = args
			}
			for i, _ := range origins {
				origins[i] = middleware.Host(origins[i]).Normalize()
				z[origins[i]] = file.NewZone(origins[i])
				names = append(names, origins[i])
			}

			for c.NextBlock() {
				t, f, e := parseTransfer(c)
				if e != nil {
					return file.Zones{}, e
				}
				for _, origin := range origins {
					if t != nil {
						z[origin].TransferTo = append(z[origin].TransferTo, t...)
					}
					if f != nil {
						z[origin].TransferFrom = append(z[origin].TransferFrom, f...)
					}
				}
			}
		}
	}
	return file.Zones{Z: z, Names: names}, nil
}
