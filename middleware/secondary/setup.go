package secondary

import (
	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("secondary", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	zones, err := secondaryParse(c)
	if err != nil {
		return middleware.Error("secondary", err)
	}

	// Add startup functions to retrieve the zone and keep it up to date.
	for _, n := range zones.Names {
		if len(zones.Z[n].TransferFrom) > 0 {
			c.OnStartup(func() error {
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

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return Secondary{file.File{Next: next, Zones: zones}}
	})

	return nil
}

func secondaryParse(c *caddy.Controller) (file.Zones, error) {
	z := make(map[string]*file.Zone)
	names := []string{}
	origins := []string{}
	for c.Next() {
		if c.Val() == "secondary" {
			// secondary [origin]
			origins = make([]string, len(c.ServerBlockKeys))
			copy(origins, c.ServerBlockKeys)
			args := c.RemainingArgs()
			if len(args) > 0 {
				origins = args
			}
			for i := range origins {
				origins[i] = middleware.Host(origins[i]).Normalize()
				z[origins[i]] = file.NewZone(origins[i], "stdin")
				names = append(names, origins[i])
			}

			for c.NextBlock() {
				t, f, e := file.TransferParse(c, true)
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
