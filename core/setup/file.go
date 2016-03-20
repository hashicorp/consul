package setup

import (
	"log"
	"os"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file"

	"github.com/miekg/dns"
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
	// Maybe multiple, each for each zone.
	z := make(map[string]file.Zone)
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
				c.Next()
				origin = c.Val()
			}
			// normalize this origin
			origin = middleware.Host(origin).StandardHost()

			zone, err := parseZone(origin, fileName)
			if err == nil {
				z[origin] = zone
			}
			names = append(names, origin)
		}
	}
	return file.Zones{Z: z, Names: names}, nil
}

//
// parsrZone parses the zone in filename and returns a []RR or an error.
func parseZone(origin, fileName string) (file.Zone, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	tokens := dns.ParseZone(f, origin, fileName)
	zone := make([]dns.RR, 0, defaultZoneSize)
	for x := range tokens {
		if x.Error != nil {
			log.Printf("[ERROR] failed to parse %s: %v", origin, x.Error)
			return nil, x.Error
		}
		zone = append(zone, x.RR)
	}
	return file.Zone(zone), nil
}

const defaultZoneSize = 20 // A made up number.
