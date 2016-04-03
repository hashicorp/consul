package setup

import (
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
					z[origin].TransferTo = append(z[origin].TransferTo, t)
				}
			}
		}
	}
	return file.Zones{Z: z, Names: names}, nil
}

// transfer to [address]
func parseTransfer(c *Controller) (to, from string, err error) {
	what := c.Val()
	if !c.NextArg() {
		return "", "", c.ArgErr()
	}
	value := c.Val()
	switch what {
	case "transfer":
		if !c.NextArg() {
			return "", "", c.ArgErr()
		}
		if value == "to" {
			to = c.Val()
			to = middleware.Addr(to).Normalize()
		}
		if value == "from" {
			from = c.Val()
			from = middleware.Addr(from).Normalize()
		}
	}
	return
}
