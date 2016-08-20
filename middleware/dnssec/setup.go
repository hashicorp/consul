package dnssec

import (
	"strings"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("dnssec", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	zones, keys, err := dnssecParse(c)
	if err != nil {
		return err
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next dnsserver.Handler) dnsserver.Handler {
		return New(zones, keys, next)
	})

	return nil
}

func dnssecParse(c *caddy.Controller) ([]string, []*DNSKEY, error) {
	zones := []string{}

	keys := []*DNSKEY{}
	for c.Next() {
		if c.Val() == "dnssec" {
			// dnssec [zones...]
			zones = make([]string, len(c.ServerBlockKeys))
			copy(zones, c.ServerBlockKeys)
			args := c.RemainingArgs()
			if len(args) > 0 {
				zones = args
			}

			for c.NextBlock() {
				k, e := keyParse(c)
				if e != nil {
					return nil, nil, e
				}
				keys = append(keys, k...)
			}
		}
	}
	for i, _ := range zones {
		zones[i] = middleware.Host(zones[i]).Normalize()
	}
	return zones, keys, nil
}

func keyParse(c *caddy.Controller) ([]*DNSKEY, error) {
	keys := []*DNSKEY{}

	what := c.Val()
	if !c.NextArg() {
		return nil, c.ArgErr()
	}
	value := c.Val()
	switch what {
	case "key":
		if value == "file" {
			ks := c.RemainingArgs()
			for _, k := range ks {
				base := k
				// Kmiek.nl.+013+26205.key, handle .private or without extension: Kmiek.nl.+013+26205
				if strings.HasSuffix(k, ".key") {
					base = k[:len(k)-4]
				}
				if strings.HasSuffix(k, ".private") {
					base = k[:len(k)-8]
				}
				k, err := ParseKeyFile(base+".key", base+".private")
				if err != nil {
					return nil, err
				}
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}
