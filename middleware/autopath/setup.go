package autopath

import (
	"fmt"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/erratic"
	"github.com/coredns/coredns/middleware/kubernetes"

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

	// Do this in OnStartup, so all middleware has been initialized.
	c.OnStartup(func() error {
		// TODO(miek): fabricate test to proof this is not thread safe.
		m := dnsserver.GetConfig(c).GetHandler(mw)
		if m == nil {
			return nil
		}
		if x, ok := m.(kubernetes.Kubernetes); ok {
			ap.searchFunc = x.AutoPath
		}
		if x, ok := m.(*erratic.Erratic); ok {
			ap.searchFunc = x.AutoPath
		}
		return nil
	})

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		ap.Next = next
		return ap
	})

	return nil
}

// allowedMiddleware has a list of middleware that can be used by autopath. For this to work, they
// need to register themselves with dnsserver.RegisterHandler.
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
