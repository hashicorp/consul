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
	keys := map[string]string{}
	credential := credentials.NewEnvCredentials()
	var fall fall.F

	up, _ := upstream.New(nil)
	for c.Next() {
		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return c.Errf("invalid zone '%s'", args[i])
			}
			if parts[0] == "" || parts[1] == "" {
				return c.Errf("invalid zone '%s'", args[i])
			}
			zone := plugin.Host(parts[0]).Normalize()
			if v, ok := keys[zone]; ok && v != parts[1] {
				return c.Errf("conflict zone '%s' ('%s' vs. '%s')", zone, v, parts[1])
			}
			keys[zone] = parts[1]
		}

		for c.NextBlock() {
			switch c.Val() {
			case "aws_access_key":
				v := c.RemainingArgs()
				if len(v) < 2 {
					return c.Errf("invalid access key '%v'", v)
				}
				credential = credentials.NewStaticCredentials(v[0], v[1], "")
			case "upstream":
				args := c.RemainingArgs()
				// TODO(dilyevsky): There is a bug that causes coredns to crash
				// when no upstream endpoint is provided.
				if len(args) == 0 {
					return c.Errf("local upstream not supported. please provide upstream endpoint")
				}
				var err error
				up, err = upstream.New(args)
				if err != nil {
					return c.Errf("invalid upstream: %v", err)
				}
			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	client := f(credential)
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
