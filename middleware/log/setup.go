package log

import (
	"fmt"
	"log"
	"os"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/response"

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
		return middleware.Error("log", err)
	}

	// Open the log files for writing when the server starts
	c.OnStartup(func() error {
		for i := 0; i < len(rules); i++ {
			// We only support stdout
			writer := os.Stdout
			if rules[i].OutputFile != "stdout" {
				return middleware.Error("log", fmt.Errorf("invalid log file: %s", rules[i].OutputFile))
			}

			rules[i].Log = log.New(writer, "", 0)
		}

		return nil
	})

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
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
				NameScope:  ".",
				OutputFile: DefaultLogFilename,
				Format:     DefaultLogFormat,
			})
		} else if len(args) == 1 {
			// Only an output file specified.
			rules = append(rules, Rule{
				NameScope:  ".",
				OutputFile: args[0],
				Format:     DefaultLogFormat,
			})
		} else {
			// Name scope, output file, and maybe a format specified

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

			rules = append(rules, Rule{
				NameScope:  dns.Fqdn(args[0]),
				OutputFile: args[1],
				Format:     format,
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
