package autopath

import (
	"fmt"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"

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
		return middleware.Error("autopath", err)
	}

	c.OnStartup(func() error {
		// So we know for sure the mw is initialized.
		m := dnsserver.GetMiddleware(c, mw)
		switch mw {
		case "kubernetes":
			m = m
			//if k, ok := m.(kubernetes.Kubernetes); ok {
			//&ap.searchFunc = k.AutoPath
			//}
		}
		return nil
	})

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		ap.Next = next
		return ap
	})

	return nil
}

var allowedMiddleware = map[string]bool{
	"@kubernetes": true,
}

func autoPathParse(c *caddy.Controller) (*AutoPath, string, error) {
	ap := new(AutoPath)
	mw := ""

	for c.Next() {
		zoneAndresolv := c.RemainingArgs()
		if len(zoneAndresolv) < 1 {
			return nil, "", fmt.Errorf("no resolv-conf specified")
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
				return nil, "", fmt.Errorf("failed to parse %q: %v", resolv, err)
			}
			ap.search = rc.Search
			middleware.Zones(ap.search).Normalize()
			ap.search = append(ap.search, "") // sentinal value as demanded.
		}
		ap.Zones = zoneAndresolv[:len(zoneAndresolv)-1]
		if len(ap.Zones) == 0 {
			ap.Zones = make([]string, len(c.ServerBlockKeys))
			copy(ap.Zones, c.ServerBlockKeys)
		}
		for i, str := range ap.Zones {
			ap.Zones[i] = middleware.Host(str).Normalize()
		}
	}
	return ap, mw, nil
}
