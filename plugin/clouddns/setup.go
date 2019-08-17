package clouddns

import (
	"context"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/caddyserver/caddy"
	gcp "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

var log = clog.NewWithPlugin("clouddns")

func init() {
	caddy.RegisterPlugin("clouddns", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(ctx context.Context, opt option.ClientOption) (gcpDNS, error) {
				var err error
				var client *gcp.Service
				if opt != nil {
					client, err = gcp.NewService(ctx, opt)
				} else {
					// if credentials file is not provided in the Corefile
					// authenticate the client using env variables
					client, err = gcp.NewService(ctx)
				}
				return gcpClient{client}, err
			}
			return setup(c, f)
		},
	})
}

func setup(c *caddy.Controller, f func(ctx context.Context, opt option.ClientOption) (gcpDNS, error)) error {
	for c.Next() {
		keyPairs := map[string]struct{}{}
		keys := map[string][]string{}

		var fall fall.F
		up := upstream.New()

		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 3)
			if len(parts) != 3 {
				return c.Errf("invalid zone '%s'", args[i])
			}
			dnsName, projectName, hostedZone := parts[0], parts[1], parts[2]
			if dnsName == "" || projectName == "" || hostedZone == "" {
				return c.Errf("invalid zone '%s'", args[i])
			}
			if _, ok := keyPairs[args[i]]; ok {
				return c.Errf("conflict zone '%s'", args[i])
			}

			keyPairs[args[i]] = struct{}{}
			keys[dnsName] = append(keys[dnsName], projectName+":"+hostedZone)
		}

		var opt option.ClientOption
		for c.NextBlock() {
			switch c.Val() {
			case "upstream":
				c.RemainingArgs() // eats args
			// if filepath is provided in the Corefile use it to authenticate the dns client
			case "credentials":
				if c.NextArg() {
					opt = option.WithCredentialsFile(c.Val())
				} else {
					return c.ArgErr()
				}
			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}

		ctx := context.Background()
		client, err := f(ctx, opt)
		if err != nil {
			return err
		}

		h, err := New(ctx, client, keys, up)
		if err != nil {
			return c.Errf("failed to create Cloud DNS plugin: %v", err)
		}
		h.Fall = fall

		if err := h.Run(ctx); err != nil {
			return c.Errf("failed to initialize Cloud DNS plugin: %v", err)
		}

		dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
			h.Next = next
			return h
		})
	}

	return nil
}
