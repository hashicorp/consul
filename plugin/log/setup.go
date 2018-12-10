package log

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/response"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

func init() {
	caddy.RegisterPlugin("log", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	rules, err := logParse(c)
	if err != nil {
		return plugin.Error("log", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Logger{Next: next, Rules: rules, ErrorFunc: dnsserver.DefaultErrorFunc}
	})

	return nil
}

func logParse(c *caddy.Controller) ([]Rule, error) {
	var rules []Rule

	for c.Next() {
		args := c.RemainingArgs()

		if len(args) == 0 {
			// Nothing specified; use defaults
			rules = append(rules, Rule{
				NameScope: ".",
				Format:    DefaultLogFormat,
				Class:     make(map[response.Class]struct{}),
			})
		} else if len(args) == 1 {
			rules = append(rules, Rule{
				NameScope: dns.Fqdn(args[0]),
				Format:    DefaultLogFormat,
				Class:     make(map[response.Class]struct{}),
			})
		} else {
			// Name scope, and maybe a format specified
			format := DefaultLogFormat

			switch args[1] {
			case "{common}":
				format = CommonLogFormat
			case "{combined}":
				format = CombinedLogFormat
			default:
				format = args[1]
			}

			rules = append(rules, Rule{
				NameScope: dns.Fqdn(args[0]),
				Format:    format,
				Class:     make(map[response.Class]struct{}),
			})
		}

		// Class refinements in an extra block.
		for c.NextBlock() {
			switch c.Val() {
			// class followed by combinations of all, denial, error and success.
			case "class":
				classes := c.RemainingArgs()
				if len(classes) == 0 {
					return nil, c.ArgErr()
				}
				for _, c := range classes {
					cls, err := response.ClassFromString(c)
					if err != nil {
						return nil, err
					}
					rules[len(rules)-1].Class[cls] = struct{}{}
				}
			default:
				return nil, c.ArgErr()
			}
		}
		if len(rules[len(rules)-1].Class) == 0 {
			rules[len(rules)-1].Class[response.All] = struct{}{}
		}
	}

	return rules, nil
}
