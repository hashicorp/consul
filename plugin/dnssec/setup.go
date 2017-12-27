package dnssec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/pkg/cache"

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
		return plugin.Error("dnssec", err)
	}

	ca := cache.New(capacity)
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return New(zones, keys, next, ca)
	})

	c.OnStartup(func() error {
		once.Do(func() {
			m := dnsserver.GetConfig(c).Handler("prometheus")
			if m == nil {
				return
			}
			if x, ok := m.(*metrics.Metrics); ok {
				x.MustRegister(cacheSize)
				x.MustRegister(cacheCapacity)
				x.MustRegister(cacheHits)
				x.MustRegister(cacheMisses)
			}
		})
		return nil
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
	for i := range zones {
		zones[i] = plugin.Host(zones[i]).Normalize()
	}

	// Check if each keys owner name can actually sign the zones we want them to sign.
	for _, k := range keys {
		kname := plugin.Name(k.K.Header().Name)
		ok := false
		for i := range zones {
			if kname.Matches(zones[i]) {
				ok = true
				break
			}
		}
		if !ok {
			return zones, keys, capacity, fmt.Errorf("key %s (keyid: %d) can not sign any of the zones", string(kname), k.tag)
		}
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
		if len(ks) == 0 {
			return nil, c.ArgErr()
		}

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
