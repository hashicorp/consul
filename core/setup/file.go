package setup

import (
	"fmt"
	"net"
	"os"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file"
)

// File sets up the file middleware.
func File(c *Controller) (middleware.Middleware, error) {
	zones, err := fileParse(c)
	if err != nil {
		return nil, err
	}

	// Add startup functions to notify the master.
	for _, n := range zones.Names {
		if len(zones.Z[n].TransferTo) > 0 {
			c.Startup = append(c.Startup, func() error {
				zones.Z[n].StartupOnce.Do(func() {
					if len(zones.Z[n].TransferTo) > 0 {
						zones.Z[n].Notify()
					}
				})
				return nil
			})
		}
	}

	return func(next middleware.Handler) middleware.Handler {
		return file.File{Next: next, Zones: zones}
	}, nil

}

func fileParse(c *Controller) (file.Zones, error) {
	z := make(map[string]*file.Zone)
	names := []string{}
	for c.Next() {
		if c.Val() == "file" {
			// file db.file [origin]
			if !c.NextArg() {
				return file.Zones{}, c.ArgErr()
			}
			fileName := c.Val()

			origins := []string{c.ServerBlockHosts[c.ServerBlockHostIndex]}
			args := c.RemainingArgs()
			if len(args) > 0 {
				origins = args
			}

			reader, err := os.Open(fileName)
			if err != nil {
				return file.Zones{}, err
			}

			for i, _ := range origins {
				origins[i] = middleware.Host(origins[i]).Normalize()
				zone, err := file.Parse(reader, origins[i], fileName)
				if err == nil {
					z[origins[i]] = zone
				}
				names = append(names, origins[i])
			}

			for c.NextBlock() {
				t, _, e := parseTransfer(c)
				if e != nil {
					return file.Zones{}, e
				}
				// discard from, here, maybe check and show log when we do?
				for _, origin := range origins {
					if t != nil {
						z[origin].TransferTo = append(z[origin].TransferTo, t...)
					}
				}
			}
		}
	}
	return file.Zones{Z: z, Names: names}, nil
}

// transfer to [address...]
func parseTransfer(c *Controller) (tos, froms []string, err error) {
	what := c.Val()
	if !c.NextArg() {
		return nil, nil, c.ArgErr()
	}
	value := c.Val()
	switch what {
	case "transfer":
		if !c.NextArg() {
			return nil, nil, c.ArgErr()
		}
		if value == "to" {
			tos := c.RemainingArgs()
			for i, _ := range tos {
				if x := net.ParseIP(tos[i]); x == nil {
					return nil, nil, fmt.Errorf("must specify an IP addres: `%s'", tos[i])
				}
				if tos[i] != "*" {
					tos[i] = middleware.Addr(tos[i]).Normalize()
				}
			}
		}
		if value == "from" {
			froms := c.RemainingArgs()
			for i, _ := range froms {
				if x := net.ParseIP(froms[i]); x == nil {
					return nil, nil, fmt.Errorf("must specify an IP addres: `%s'", froms[i])
				}
				if froms[i] != "*" {
					froms[i] = middleware.Addr(froms[i]).Normalize()
				} else {
					return nil, nil, fmt.Errorf("can't use '*' in transfer from")
				}
			}
		}
	}
	return
}
