package file

import (
	"fmt"
	"net"
	"os"
	"path"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	"github.com/miekg/coredns/middleware/proxy"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("file", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	zones, err := fileParse(c)
	if err != nil {
		return middleware.Error("file", err)
	}

	// Add startup functions to notify the master(s).
	for _, n := range zones.Names {
		z := zones.Z[n]
		c.OnStartup(func() error {
			z.StartupOnce.Do(func() {
				if len(z.TransferTo) > 0 {
					z.Notify()
				}
				z.Reload()
			})
			return nil
		})
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return File{Next: next, Zones: zones}
	})

	return nil
}

func fileParse(c *caddy.Controller) (Zones, error) {
	z := make(map[string]*Zone)
	names := []string{}
	origins := []string{}

	config := dnsserver.GetConfig(c)

	for c.Next() {
		if c.Val() == "file" {
			// file db.file [zones...]
			if !c.NextArg() {
				return Zones{}, c.ArgErr()
			}
			fileName := c.Val()

			origins = make([]string, len(c.ServerBlockKeys))
			copy(origins, c.ServerBlockKeys)
			args := c.RemainingArgs()
			if len(args) > 0 {
				origins = args
			}

			if !path.IsAbs(fileName) && config.Root != "" {
				fileName = path.Join(config.Root, fileName)
			}

			reader, err := os.Open(fileName)
			if err != nil {
				// bail out
				return Zones{}, err
			}

			for i := range origins {
				origins[i] = middleware.Host(origins[i]).Normalize()
				zone, err := Parse(reader, origins[i], fileName)
				if err == nil {
					z[origins[i]] = zone
				} else {
					return Zones{}, err
				}
				names = append(names, origins[i])
			}

			noReload := false
			prxy := proxy.Proxy{}
			for c.NextBlock() {
				t, _, e := TransferParse(c, false)
				if e != nil {
					return Zones{}, e
				}
				switch c.Val() {
				case "no_reload":
					noReload = true

				case "upstream":
					args := c.RemainingArgs()
					if len(args) == 0 {
						return Zones{}, c.ArgErr()
					}
					for i := 0; i < len(args); i++ {
						h, p, e := net.SplitHostPort(args[i])
						if e != nil && p == "" {
							args[i] = h + ":53"
						}
					}
					prxy = proxy.New(args)
				}

				for _, origin := range origins {
					if t != nil {
						z[origin].TransferTo = append(z[origin].TransferTo, t...)
					}
					z[origin].NoReload = noReload
					z[origin].Proxy = prxy
				}
			}
		}
	}
	return Zones{Z: z, Names: names}, nil
}

// TransferParse parses transfer statements: 'transfer to [address...]'.
func TransferParse(c *caddy.Controller, secondary bool) (tos, froms []string, err error) {
	what := c.Val()
	if !c.NextArg() {
		return nil, nil, c.ArgErr()
	}
	value := c.Val()
	switch what {
	case "transfer":
		if value == "to" {
			tos = c.RemainingArgs()
			for i := range tos {
				if tos[i] != "*" {
					normalized, err := dnsutil.ParseHostPort(tos[i], "53")
					if err != nil {
						return nil, nil, err
					}
					tos[i] = normalized
				}
			}
		}
		if value == "from" {
			if !secondary {
				return nil, nil, fmt.Errorf("can't use `transfer from` when not being a secondary")
			}
			froms = c.RemainingArgs()
			for i := range froms {
				if froms[i] != "*" {
					normalized, err := dnsutil.ParseHostPort(froms[i], "53")
					if err != nil {
						return nil, nil, err
					}
					froms[i] = normalized
				} else {
					return nil, nil, fmt.Errorf("can't use '*' in transfer from")
				}
			}
		}
	}
	return
}
