package dnssec

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/pkg/cache"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/mholt/caddy"
)

var log = clog.NewWithPlugin("dnssec")

func init() {
	caddy.RegisterPlugin("dnssec", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	zones, keys, capacity, splitkeys, err := dnssecParse(c)
	if err != nil {
		return plugin.Error("dnssec", err)
	}

	ca := cache.New(capacity)
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return New(zones, keys, splitkeys, next, ca)
	})

	c.OnStartup(func() error {
		metrics.MustRegister(c, cacheSize, cacheHits, cacheMisses)
		return nil
	})

	return nil
}

func dnssecParse(c *caddy.Controller) ([]string, []*DNSKEY, int, bool, error) {
	zones := []string{}

	keys := []*DNSKEY{}

	capacity := defaultCap

	i := 0
	for c.Next() {
		if i > 0 {
			return nil, nil, 0, false, plugin.ErrOnce
		}
		i++

		// dnssec [zones...]
		zones = make([]string, len(c.ServerBlockKeys))
		copy(zones, c.ServerBlockKeys)
		args := c.RemainingArgs()
		if len(args) > 0 {
			zones = args
		}

		for c.NextBlock() {

			switch x := c.Val(); x {
			case "key":
				k, e := keyParse(c)
				if e != nil {
					return nil, nil, 0, false, e
				}
				keys = append(keys, k...)
			case "cache_capacity":
				if !c.NextArg() {
					return nil, nil, 0, false, c.ArgErr()
				}
				value := c.Val()
				cacheCap, err := strconv.Atoi(value)
				if err != nil {
					return nil, nil, 0, false, err
				}
				capacity = cacheCap
			default:
				return nil, nil, 0, false, c.Errf("unknown property '%s'", x)
			}

		}
	}
	for i := range zones {
		zones[i] = plugin.Host(zones[i]).Normalize()
	}

	// Check if we have both KSKs and ZSKs.
	zsk, ksk := 0, 0
	for _, k := range keys {
		if k.isKSK() {
			ksk++
		} else if k.isZSK() {
			zsk++
		}
	}
	splitkeys := zsk > 0 && ksk > 0

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
			return zones, keys, capacity, splitkeys, fmt.Errorf("key %s (keyid: %d) can not sign any of the zones", string(kname), k.tag)
		}
	}

	return zones, keys, capacity, splitkeys, nil
}

func keyParse(c *caddy.Controller) ([]*DNSKEY, error) {
	keys := []*DNSKEY{}
	config := dnsserver.GetConfig(c)

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
			if !filepath.IsAbs(base) && config.Root != "" {
				base = filepath.Join(config.Root, base)
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
