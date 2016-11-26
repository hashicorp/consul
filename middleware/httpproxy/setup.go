package httpproxy

import (
	"fmt"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyfile"
)

func init() {
	caddy.RegisterPlugin("httpproxy", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	p, err := httpproxyParse(c)
	if err != nil {
		return middleware.Error("httpproxy", err)
	}

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		p.Next = next
		return p
	})

	c.OnStartup(func() error {
		OnStartupMetrics()
		e := p.e.OnStartup()
		if e != nil {
			return middleware.Error("httpproxy", e)
		}
		return nil
	})
	c.OnShutdown(func() error {
		e := p.e.OnShutdown()
		if e != nil {
			return middleware.Error("httpproxy", e)
		}
		return nil
	})

	return nil
}

func httpproxyParse(c *caddy.Controller) (*Proxy, error) {
	var p = &Proxy{}

	for c.Next() {
		if !c.Args(&p.from) {
			return p, c.ArgErr()
		}
		to := c.RemainingArgs()
		if len(to) != 1 {
			return p, c.ArgErr()
		}
		switch to[0] {
		case "dns.google.com":
			p.e = newGoogle()
			u, _ := newSimpleUpstream([]string{"8.8.8.8:53", "8.8.4.4:53"})
			p.e.SetUpstream(u)
		default:
			return p, fmt.Errorf("unknown http proxy %q", to[0])
		}

		for c.NextBlock() {
			if err := parseBlock(&c.Dispenser, p); err != nil {
				return p, err
			}
		}
	}

	return p, nil
}

func parseBlock(c *caddyfile.Dispenser, p *Proxy) error {
	switch c.Val() {
	case "upstream":
		upstreams := c.RemainingArgs()
		if len(upstreams) == 0 {
			return c.ArgErr()
		}
		u, err := newSimpleUpstream(upstreams)
		if err != nil {
			return err
		}
		p.e.SetUpstream(u)
	default:
		return c.Errf("unknown property '%s'", c.Val())
	}
	return nil
}
