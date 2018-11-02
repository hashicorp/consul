package route53

import (
	"context"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/mholt/caddy"
)

var log = clog.NewWithPlugin("route53")

func init() {
	caddy.RegisterPlugin("route53", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(credential *credentials.Credentials) route53iface.Route53API {
				return route53.New(session.Must(session.NewSession(&aws.Config{
					Credentials: credential,
				})))
			}
			return setup(c, f)
		},
	})
}

func setup(c *caddy.Controller, f func(*credentials.Credentials) route53iface.Route53API) error {
	keyPairs := map[string]struct{}{}
	keys := map[string][]string{}

	// Route53 plugin attempts to find AWS credentials by using ChainCredentials.
	// And the order of that provider chain is as follows:
	// Static AWS keys -> Environment Variables -> Credentials file -> IAM role
	// With that said, even though a user doesn't define any credentials in
	// Corefile, we should still attempt to read the default credentials file,
	// ~/.aws/credentials with the default profile.
	sharedProvider := &credentials.SharedCredentialsProvider{}
	var providers []credentials.Provider
	var fall fall.F

	up, _ := upstream.New(nil)
	for c.Next() {
		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return c.Errf("invalid zone '%s'", args[i])
			}
			dns, hostedZoneID := parts[0], parts[1]
			if dns == "" || hostedZoneID == "" {
				return c.Errf("invalid zone '%s'", args[i])
			}
			if _, ok := keyPairs[args[i]]; ok {
				return c.Errf("conflict zone '%s'", args[i])
			}

			keyPairs[args[i]] = struct{}{}
			keys[dns] = append(keys[dns], hostedZoneID)
		}

		for c.NextBlock() {
			switch c.Val() {
			case "aws_access_key":
				v := c.RemainingArgs()
				if len(v) < 2 {
					return c.Errf("invalid access key '%v'", v)
				}
				providers = append(providers, &credentials.StaticProvider{
					Value: credentials.Value{
						AccessKeyID:     v[0],
						SecretAccessKey: v[1],
					},
				})
			case "upstream":
				args := c.RemainingArgs()
				var err error
				up, err = upstream.New(args)
				if err != nil {
					return c.Errf("invalid upstream: %v", err)
				}
			case "credentials":
				if c.NextArg() {
					sharedProvider.Profile = c.Val()
				} else {
					return c.ArgErr()
				}
				if c.NextArg() {
					sharedProvider.Filename = c.Val()
				}
			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	providers = append(providers, &credentials.EnvProvider{}, sharedProvider)

	client := f(credentials.NewChainCredentials(providers))
	ctx := context.Background()
	h, err := New(ctx, client, keys, &up)
	if err != nil {
		return c.Errf("failed to create Route53 plugin: %v", err)
	}
	h.Fall = fall
	if err := h.Run(ctx); err != nil {
		return c.Errf("failed to initialize Route53 plugin: %v", err)
	}
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})

	return nil
}
