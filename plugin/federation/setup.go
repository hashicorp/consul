package federation

import (
	"fmt"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/miekg/dns"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("federation", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	fed, err := federationParse(c)
	if err != nil {
		return plugin.Error("federation", err)
	}

	// Do this in OnStartup, so all plugin has been initialized.
	c.OnStartup(func() error {
		m := dnsserver.GetConfig(c).Handler("kubernetes")
		if m == nil {
			return nil
		}
		if x, ok := m.(*kubernetes.Kubernetes); ok {
			fed.Federations = x.Federations
		}
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		fed.Next = next
		return fed
	})

	return nil
}

func federationParse(c *caddy.Controller) (*Federation, error) {
	fed := New()

	for c.Next() {
		// federation [zones..]
		zones := c.RemainingArgs()
		var origins []string
		if len(zones) > 0 {
			origins = make([]string, len(zones))
			copy(origins, zones)
		} else {
			origins = make([]string, len(c.ServerBlockKeys))
			copy(origins, c.ServerBlockKeys)
		}

		for c.NextBlock() {
			x := c.Val()
			switch x {
			case "upstream":
				args := c.RemainingArgs()
				u, err := upstream.New(args)
				if err != nil {
					return nil, err
				}
				fed.Upstream = &u
			default:
				args := c.RemainingArgs()
				if x := len(args); x != 1 {
					return fed, fmt.Errorf("need two arguments for federation, got %d", x)
				}

				fed.f[x] = dns.Fqdn(args[0])
			}
		}

		for i := range origins {
			origins[i] = plugin.Host(origins[i]).Normalize()
		}

		fed.zones = origins

		if len(fed.f) == 0 {
			return fed, fmt.Errorf("at least one name to zone federation expected")
		}

		return fed, nil
	}

	return fed, nil
}
