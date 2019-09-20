package route53

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/caddyserver/caddy"
)

var log = clog.NewWithPlugin("route53")

func init() {
	plugin.Register("route53",
		func(c *caddy.Controller) error {
			f := func(credential *credentials.Credentials) route53iface.Route53API {
				return route53.New(session.Must(session.NewSession(&aws.Config{
					Credentials: credential,
				})))
			}
			return setup(c, f)
		},
	)
}

func setup(c *caddy.Controller, f func(*credentials.Credentials) route53iface.Route53API) error {
	for c.Next() {
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

		refresh := time.Duration(1) * time.Minute // default update frequency to 1 minute

		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return plugin.Error("route53", c.Errf("invalid zone '%s'", args[i]))
			}
			dns, hostedZoneID := parts[0], parts[1]
			if dns == "" || hostedZoneID == "" {
				return plugin.Error("route53", c.Errf("invalid zone '%s'", args[i]))
			}
			if _, ok := keyPairs[args[i]]; ok {
				return plugin.Error("route53", c.Errf("conflict zone '%s'", args[i]))
			}

			keyPairs[args[i]] = struct{}{}
			keys[dns] = append(keys[dns], hostedZoneID)
		}

		for c.NextBlock() {
			switch c.Val() {
			case "aws_access_key":
				v := c.RemainingArgs()
				if len(v) < 2 {
					return plugin.Error("route53", c.Errf("invalid access key '%v'", v))
				}
				providers = append(providers, &credentials.StaticProvider{
					Value: credentials.Value{
						AccessKeyID:     v[0],
						SecretAccessKey: v[1],
					},
				})
			case "upstream":
				c.RemainingArgs() // eats args
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
			case "refresh":
				if c.NextArg() {
					refreshStr := c.Val()
					_, err := strconv.Atoi(refreshStr)
					if err == nil {
						refreshStr = fmt.Sprintf("%ss", c.Val())
					}
					refresh, err = time.ParseDuration(refreshStr)
					if err != nil {
						return plugin.Error("route53", c.Errf("Unable to parse duration: '%v'", err))
					}
					if refresh <= 0 {
						return plugin.Error("route53", c.Errf("refresh interval must be greater than 0: %s", refreshStr))
					}
				} else {
					return plugin.Error("route53", c.ArgErr())
				}
			default:
				return plugin.Error("route53", c.Errf("unknown property '%s'", c.Val()))
			}
		}
		providers = append(providers, &credentials.EnvProvider{}, sharedProvider, &ec2rolecreds.EC2RoleProvider{
			Client: ec2metadata.New(session.New(&aws.Config{})),
		})
		client := f(credentials.NewChainCredentials(providers))
		ctx := context.Background()
		h, err := New(ctx, client, keys, refresh)
		if err != nil {
			return plugin.Error("route53", c.Errf("failed to create Route53 plugin: %v", err))
		}
		h.Fall = fall
		if err := h.Run(ctx); err != nil {
			return plugin.Error("route53", c.Errf("failed to initialize Route53 plugin: %v", err))
		}
		dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
			h.Next = next
			return h
		})
	}
	return nil
}
