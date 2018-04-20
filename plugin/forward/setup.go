package forward

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	pkgtls "github.com/coredns/coredns/plugin/pkg/tls"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("forward", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	f, err := parseForward(c)
	if err != nil {
		return plugin.Error("forward", err)
	}
	if f.Len() > max {
		return plugin.Error("forward", fmt.Errorf("more than %d TOs configured: %d", max, f.Len()))
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		f.Next = next
		return f
	})

	c.OnStartup(func() error {
		once.Do(func() {
			metrics.MustRegister(c, RequestCount, RcodeCount, RequestDuration, HealthcheckFailureCount, SocketGauge)
		})
		return f.OnStartup()
	})

	c.OnShutdown(func() error {
		return f.OnShutdown()
	})

	return nil
}

// OnStartup starts a goroutines for all proxies.
func (f *Forward) OnStartup() (err error) {
	for _, p := range f.proxies {
		p.start(f.hcInterval)
	}
	return nil
}

// OnShutdown stops all configured proxies.
func (f *Forward) OnShutdown() error {
	for _, p := range f.proxies {
		p.close()
	}
	return nil
}

// Close is a synonym for OnShutdown().
func (f *Forward) Close() { f.OnShutdown() }

func parseForward(c *caddy.Controller) (*Forward, error) {
	f := New()

	protocols := map[int]int{}

	i := 0
	for c.Next() {
		if i > 0 {
			return nil, plugin.ErrOnce
		}
		i++

		if !c.Args(&f.from) {
			return f, c.ArgErr()
		}
		f.from = plugin.Host(f.from).Normalize()

		to := c.RemainingArgs()
		if len(to) == 0 {
			return f, c.ArgErr()
		}

		// A bit fiddly, but first check if we've got protocols and if so add them back in when we create the proxies.
		protocols = make(map[int]int)
		for i := range to {
			protocols[i], to[i] = protocol(to[i])
		}

		// If parseHostPortOrFile expands a file with a lot of nameserver our accounting in protocols doesn't make
		// any sense anymore... For now: lets don't care.
		toHosts, err := dnsutil.ParseHostPortOrFile(to...)
		if err != nil {
			return f, err
		}

		for i, h := range toHosts {
			// Double check the port, if e.g. is 53 and the transport is TLS make it 853.
			// This can be somewhat annoying because you *can't* have TLS on port 53 then.
			switch protocols[i] {
			case TLS:
				h1, p, err := net.SplitHostPort(h)
				if err != nil {
					break
				}

				// This is more of a bug in dnsutil.ParseHostPortOrFile that defaults to
				// 53 because it doesn't know about the tls:// // and friends (that should be fixed). Hence
				// Fix the port number here, back to what the user intended.
				if p == "53" {
					h = net.JoinHostPort(h1, "853")
				}
			}

			// We can't set tlsConfig here, because we haven't parsed it yet.
			// We set it below at the end of parseBlock, use nil now.
			p := NewProxy(h, nil /* no TLS */)
			f.proxies = append(f.proxies, p)
		}

		for c.NextBlock() {
			if err := parseBlock(c, f); err != nil {
				return f, err
			}
		}
	}

	if f.tlsServerName != "" {
		f.tlsConfig.ServerName = f.tlsServerName
	}
	for i := range f.proxies {
		// Only set this for proxies that need it.
		if protocols[i] == TLS {
			f.proxies[i].SetTLSConfig(f.tlsConfig)
		}
		f.proxies[i].SetExpire(f.expire)
	}
	return f, nil
}

func parseBlock(c *caddy.Controller, f *Forward) error {
	switch c.Val() {
	case "except":
		ignore := c.RemainingArgs()
		if len(ignore) == 0 {
			return c.ArgErr()
		}
		for i := 0; i < len(ignore); i++ {
			ignore[i] = plugin.Host(ignore[i]).Normalize()
		}
		f.ignored = ignore
	case "max_fails":
		if !c.NextArg() {
			return c.ArgErr()
		}
		n, err := strconv.Atoi(c.Val())
		if err != nil {
			return err
		}
		if n < 0 {
			return fmt.Errorf("max_fails can't be negative: %d", n)
		}
		f.maxfails = uint32(n)
	case "health_check":
		if !c.NextArg() {
			return c.ArgErr()
		}
		dur, err := time.ParseDuration(c.Val())
		if err != nil {
			return err
		}
		if dur < 0 {
			return fmt.Errorf("health_check can't be negative: %d", dur)
		}
		f.hcInterval = dur
	case "force_tcp":
		if c.NextArg() {
			return c.ArgErr()
		}
		f.forceTCP = true
	case "tls":
		args := c.RemainingArgs()
		if len(args) > 3 {
			return c.ArgErr()
		}

		tlsConfig, err := pkgtls.NewTLSConfigFromArgs(args...)
		if err != nil {
			return err
		}
		f.tlsConfig = tlsConfig
	case "tls_servername":
		if !c.NextArg() {
			return c.ArgErr()
		}
		f.tlsServerName = c.Val()
	case "expire":
		if !c.NextArg() {
			return c.ArgErr()
		}
		dur, err := time.ParseDuration(c.Val())
		if err != nil {
			return err
		}
		if dur < 0 {
			return fmt.Errorf("expire can't be negative: %s", dur)
		}
		f.expire = dur
	case "policy":
		if !c.NextArg() {
			return c.ArgErr()
		}
		switch x := c.Val(); x {
		case "random":
			f.p = &random{}
		case "round_robin":
			f.p = &roundRobin{}
		case "sequential":
			f.p = &sequential{}
		default:
			return c.Errf("unknown policy '%s'", x)
		}

	default:
		return c.Errf("unknown property '%s'", c.Val())
	}

	return nil
}

const max = 15 // Maximum number of upstreams.
