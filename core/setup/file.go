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
	// Set start function is transfer is specified

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

			origin := c.ServerBlockHosts[c.ServerBlockHostIndex]
			if c.NextArg() {
				origin = c.Val()
			}
			origin = middleware.Host(origin).Normalize()

			// TODO(miek): we should allow more. Issue #54.
			reader, err := os.Open(fileName)
			if err != nil {
				return file.Zones{}, err
			}
			zone, err := file.Parse(reader, origin, fileName)
			if err == nil {
				z[origin] = zone
			}
			names = append(names, origin)

			for c.NextBlock() {
				t, _, e := parseTransfer(c)
				if e != nil {
					return file.Zones{}, e
				}
				// discard from, here, maybe check and show log when we do?
				z[origin].TransferTo = append(z[origin].TransferTo, t)
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
