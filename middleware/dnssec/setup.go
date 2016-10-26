package dnssec

import (
	"strconv"
	"strings"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/hashicorp/golang-lru"
	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("dnssec", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	zones, keys, capacity, err := dnssecParse(c)
	if err != nil {
		return middleware.Error("dnssec", err)
	}

	cache, err := lru.New(capacity)
	if err != nil {
		return err
	}
	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		return New(zones, keys, next, cache)
	})

	// Export the capacity for the metrics. This only happens once, because this is a re-load change only.
	cacheCapacity.WithLabelValues("signature").Set(float64(capacity))

	return nil
}

func dnssecParse(c *caddy.Controller) ([]string, []*DNSKEY, int, error) {
	zones := []string{}

	keys := []*DNSKEY{}

	capacity := defaultCap
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
				switch c.Val() {
				case "key":
					k, e := keyParse(c)
					if e != nil {
						return nil, nil, 0, e
					}
					keys = append(keys, k...)
				case "cache_capacity":
					if !c.NextArg() {
						return nil, nil, 0, c.ArgErr()
					}
					value := c.Val()
					cacheCap, err := strconv.Atoi(value)
					if err != nil {
						return nil, nil, 0, err
					}
					capacity = cacheCap
				}

			}
		}
	}
	for i := range zones {
		zones[i] = middleware.Host(zones[i]).Normalize()
	}
	return zones, keys, capacity, nil
}

func keyParse(c *caddy.Controller) ([]*DNSKEY, error) {
	keys := []*DNSKEY{}

	if !c.NextArg() {
		return nil, c.ArgErr()
	}
	value := c.Val()
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
	return keys, nil
}
