package setup

import (
	"path"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/dnssec"
)

// Dnssec sets up the dnssec middleware.
func Dnssec(c *Controller) (middleware.Middleware, error) {
	zones, keys, err := dnssecParse(c)
	if err != nil {
		return nil, err
	}

	return func(next middleware.Handler) middleware.Handler {
		return dnssec.NewDnssec(zones, keys, next)
	}, nil
}

func dnssecParse(c *Controller) ([]string, []*dnssec.DNSKEY, error) {
	zones := []string{}

	keys := []*dnssec.DNSKEY{}
	for c.Next() {
		if c.Val() == "dnssec" {
			// dnssec [zones...]
			zones = c.ServerBlockHosts
			args := c.RemainingArgs()
			if len(args) > 0 {
				zones = args
			}

			for c.NextBlock() {
				k, e := keyParse(c)
				if e != nil {
					// TODO(miek): Log and drop or something? stop startup?
					continue
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

func keyParse(c *Controller) ([]*dnssec.DNSKEY, error) {
	keys := []*dnssec.DNSKEY{}

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
				// Kmiek.nl.+013+26205.key, handle .private or without extension: Kmiek.nl.+013+26205
				ext := path.Ext(k) // TODO(miek): test things like .key
				base := k
				if len(ext) > 0 {
					base = k[:len(k)-len(ext)]
				}
				k, err := dnssec.ParseKeyFile(base+".key", base+".private")
				if err != nil {
					return nil, err
				}
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}
