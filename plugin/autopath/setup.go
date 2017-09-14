package autopath

import (
	"fmt"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/erratic"
	"github.com/coredns/coredns/plugin/kubernetes"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

func init() {
	caddy.RegisterPlugin("autopath", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})

}

func setup(c *caddy.Controller) error {
	ap, mw, err := autoPathParse(c)
	if err != nil {
		return plugin.Error("autopath", err)
	}

	// Do this in OnStartup, so all plugin has been initialized.
	c.OnStartup(func() error {
		m := dnsserver.GetConfig(c).Handler(mw)
		if m == nil {
			return nil
		}
		if x, ok := m.(*kubernetes.Kubernetes); ok {
			ap.searchFunc = x.AutoPath
		}
		if x, ok := m.(*erratic.Erratic); ok {
			ap.searchFunc = x.AutoPath
		}
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		ap.Next = next
		return ap
	})

	return nil
}

// allowedMiddleware has a list of plugin that can be used by autopath.
var allowedMiddleware = map[string]bool{
	"@kubernetes": true,
	"@erratic":    true,
}

func autoPathParse(c *caddy.Controller) (*AutoPath, string, error) {
	ap := &AutoPath{}
	mw := ""

	for c.Next() {
		zoneAndresolv := c.RemainingArgs()
		if len(zoneAndresolv) < 1 {
			return ap, "", fmt.Errorf("no resolv-conf specified")
		}
		resolv := zoneAndresolv[len(zoneAndresolv)-1]
		if resolv[0] == '@' {
			_, ok := allowedMiddleware[resolv]
			if ok {
				mw = resolv[1:]
			}
		} else {
			// assume file on disk
			rc, err := dns.ClientConfigFromFile(resolv)
			if err != nil {
				return ap, "", fmt.Errorf("failed to parse %q: %v", resolv, err)
			}
			ap.search = rc.Search
			plugin.Zones(ap.search).Normalize()
			ap.search = append(ap.search, "") // sentinal value as demanded.
		}
		ap.Zones = zoneAndresolv[:len(zoneAndresolv)-1]
		if len(ap.Zones) == 0 {
			ap.Zones = make([]string, len(c.ServerBlockKeys))
			copy(ap.Zones, c.ServerBlockKeys)
		}
		for i, str := range ap.Zones {
			ap.Zones[i] = plugin.Host(str).Normalize()
		}
	}
	return ap, mw, nil
}
