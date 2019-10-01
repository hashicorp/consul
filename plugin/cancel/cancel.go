// Package cancel implements a plugin adds a canceling context to each request.
package cancel

import (
	"context"
	"fmt"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/caddyserver/caddy"
	"github.com/miekg/dns"
)

func init() { plugin.Register("cancel", setup) }

func setup(c *caddy.Controller) error {
	ca := Cancel{}

	for c.Next() {
		args := c.RemainingArgs()
		switch len(args) {
		case 0:
			ca.timeout = 5001 * time.Millisecond
		case 1:
			dur, err := time.ParseDuration(args[0])
			if err != nil {
				return plugin.Error("cancel", fmt.Errorf("invalid duration: %q", args[0]))
			}
			if dur <= 0 {
				return plugin.Error("cancel", fmt.Errorf("invalid negative duration: %q", args[0]))
			}
			ca.timeout = dur
		default:
			return plugin.Error("cancel", c.ArgErr())
		}
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		ca.Next = next
		return ca
	})

	return nil
}

// Cancel is a plugin that adds a canceling context to each request's context.
type Cancel struct {
	timeout time.Duration
	Next    plugin.Handler
}

// ServeDNS implements the plugin.Handler interface.
func (c Cancel) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)

	code, err := plugin.NextOrFailure(c.Name(), c.Next, ctx, w, r)

	cancel()

	return code, err
}

// Name implements the Handler interface.
func (c Cancel) Name() string { return "cancel" }
