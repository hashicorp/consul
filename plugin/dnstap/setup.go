package dnstap

import (
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/dnstap/dnstapio"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/parse"

	"github.com/caddyserver/caddy"
	"github.com/caddyserver/caddy/caddyfile"
)

var log = clog.NewWithPlugin("dnstap")

func init() {
	caddy.RegisterPlugin("dnstap", caddy.Plugin{
		ServerType: "dns",
		Action:     wrapSetup,
	})
}

func wrapSetup(c *caddy.Controller) error {
	if err := setup(c); err != nil {
		return plugin.Error("dnstap", err)
	}
	return nil
}

type config struct {
	target string
	socket bool
	full   bool
}

func parseConfig(d *caddyfile.Dispenser) (c config, err error) {
	d.Next() // directive name

	if !d.Args(&c.target) {
		return c, d.ArgErr()
	}

	if strings.HasPrefix(c.target, "tcp://") {
		// remote IP endpoint
		servers, err := parse.HostPortOrFile(c.target[6:])
		if err != nil {
			return c, d.ArgErr()
		}
		c.target = servers[0]
	} else {
		// default to UNIX socket
		if strings.HasPrefix(c.target, "unix://") {
			c.target = c.target[7:]
		}
		c.socket = true
	}

	c.full = d.NextArg() && d.Val() == "full"

	return
}

func setup(c *caddy.Controller) error {
	conf, err := parseConfig(&c.Dispenser)
	if err != nil {
		return err
	}

	dio := dnstapio.New(conf.target, conf.socket)
	dnstap := Dnstap{IO: dio, JoinRawMessage: conf.full}

	c.OnStartup(func() error {
		dio.Connect()
		return nil
	})

	c.OnRestart(func() error {
		dio.Close()
		return nil
	})

	c.OnFinalShutdown(func() error {
		dio.Close()
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(
		func(next plugin.Handler) plugin.Handler {
			dnstap.Next = next
			return dnstap
		})

	return nil
}
