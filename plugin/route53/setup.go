package route53

import (
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/mholt/caddy"
)

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
	var credential *credentials.Credentials
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
			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	client := f(credential)
	zones := []string{}
	for zone, v := range keys {
		// Make sure enough credentials is needed
		if _, err := client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
			HostedZoneId: aws.String(v),
			MaxItems:     aws.String("1"),
		}); err != nil {
			return c.Errf("aws error: '%s'", err)
		}

		zones = append(zones, zone)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Route53{
			Next:   next,
			keys:   keys,
			zones:  zones,
			client: client,
		}
	})

	return nil
}
