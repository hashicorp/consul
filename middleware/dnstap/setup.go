package dnstap

import (
	"fmt"
	"log"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/dnstap/out"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyfile"
)

func init() {
	caddy.RegisterPlugin("dnstap", caddy.Plugin{
		ServerType: "dns",
		Action:     wrapSetup,
	})
}

func wrapSetup(c *caddy.Controller) error {
	if err := setup(c); err != nil {
		return middleware.Error("dnstap", err)
	}
	return nil
}

func parseConfig(c *caddyfile.Dispenser) (path string, full bool, err error) {
	c.Next() // directive name

	if !c.Args(&path) {
		err = c.ArgErr()
		return
	}

	full = c.NextArg() && c.Val() == "full"

	return
}

func setup(c *caddy.Controller) error {
	path, full, err := parseConfig(&c.Dispenser)
	if err != nil {
		return err
	}

	dnstap := Dnstap{Pack: full}

	o, err := out.NewSocket(path)
	if err != nil {
		log.Printf("[WARN] Can't connect to %s at the moment", path)
	}
	dnstap.Out = o

	c.OnShutdown(func() error {
		if err := o.Close(); err != nil {
			return fmt.Errorf("output: %s", err)
		}
		return nil
	})

	dnsserver.GetConfig(c).AddMiddleware(
		func(next middleware.Handler) middleware.Handler {
			dnstap.Next = next
			return dnstap
		})

	return nil
}
