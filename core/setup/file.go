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
			// normalize this origin
			origin = middleware.Host(origin).Standard()

			reader, err := os.Open(fileName)
			if err != nil {
				return file.Zones{}, err
			}
			zone, err := file.Parse(reader, origin, fileName)
			if err == nil {
				z[origin] = zone
			}
			names = append(names, origin)
			if c.NextBlock() {
				what := c.Val()
				if !c.NextArg() {
					return file.Zones{}, c.ArgErr()
				}
				value := c.Val()
				var err error
				switch what {
				case "transfer":
					if value == "out" {
						z[origin].Transfer.Out = true
					}
					if value == "in" {
						z[origin].Transfer.In = true
					}
				}
				if err != nil {
					return file.Zones{}, err
				}
			}
		}
	}
	return file.Zones{Z: z, Names: names}, nil
}
