package log

import (
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/replacer"
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
		return Logger{Next: next, Rules: rules, repl: replacer.New()}
	})

	return nil
}

func logParse(c *caddy.Controller) ([]Rule, error) {
	var rules []Rule

	for c.Next() {
		args := c.RemainingArgs()
		length := len(rules)

		switch len(args) {
		case 0:
			// Nothing specified; use defaults
			rules = append(rules, Rule{
				NameScope: ".",
				Format:    DefaultLogFormat,
				Class:     make(map[response.Class]struct{}),
			})
		case 1:
			rules = append(rules, Rule{
				NameScope: dns.Fqdn(args[0]),
				Format:    DefaultLogFormat,
				Class:     make(map[response.Class]struct{}),
			})
		default:
			// Name scopes, and maybe a format specified
			format := DefaultLogFormat

			if strings.Contains(args[len(args)-1], "{") {
				switch args[len(args)-1] {
				case "{common}":
					format = CommonLogFormat
				case "{combined}":
					format = CombinedLogFormat
				default:
					format = args[len(args)-1]
				}

				args = args[:len(args)-1]
			}

			for _, str := range args {
				rules = append(rules, Rule{
					NameScope: dns.Fqdn(str),
					Format:    format,
					Class:     make(map[response.Class]struct{}),
				})
			}
		}

		// Class refinements in an extra block.
		classes := make(map[response.Class]struct{})
		for c.NextBlock() {
			switch c.Val() {
			// class followed by combinations of all, denial, error and success.
			case "class":
				classesArgs := c.RemainingArgs()
				if len(classesArgs) == 0 {
					return nil, c.ArgErr()
				}
				for _, c := range classesArgs {
					cls, err := response.ClassFromString(c)
					if err != nil {
						return nil, err
					}
					classes[cls] = struct{}{}
				}
			default:
				return nil, c.ArgErr()
			}
		}
		if len(classes) == 0 {
			classes[response.All] = struct{}{}
		}

		for i := len(rules) - 1; i >= length; i-- {
			rules[i].Class = classes
		}
	}

	return rules, nil
}
