package log

import (
	"fmt"
	"log"
	"os"

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

	// Open the log files for writing when the server starts
	c.OnStartup(func() error {
		for i := 0; i < len(rules); i++ {
			rules[i].Log = log.New(os.Stdout, "", 0)
		}

		return nil
	})

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
			})
		} else if len(args) == 1 {
			// Only an output file specified, can only be *stdout*
			if args[0] != "stdout" {
				return nil, fmt.Errorf("only stdout is allowed: %s", args[0])
			}
			rules = append(rules, Rule{
				NameScope: ".",
				Format:    DefaultLogFormat,
			})
		} else {
			// Name scope, output file (stdout), and maybe a format specified

			format := DefaultLogFormat

			if len(args) > 2 {
				switch args[2] {
				case "{common}":
					format = CommonLogFormat
				case "{combined}":
					format = CombinedLogFormat
				default:
					format = args[2]
				}
			}

			if args[1] != "stdout" {
				return nil, fmt.Errorf("only stdout is allowed: %s", args[1])
			}

			rules = append(rules, Rule{
				NameScope: dns.Fqdn(args[0]),
				Format:    format,
			})
		}

		// Class refinements in an extra block.
		for c.NextBlock() {
			switch c.Val() {
			// class followed by all, denial, error or success.
			case "class":
				classes := c.RemainingArgs()
				if len(classes) == 0 {
					return nil, c.ArgErr()
				}
				cls, err := response.ClassFromString(classes[0])
				if err != nil {
					return nil, err
				}
				// update class and the last added Rule (bit icky)
				rules[len(rules)-1].Class = cls
			default:
				return nil, c.ArgErr()
			}
		}
	}

	return rules, nil
}
