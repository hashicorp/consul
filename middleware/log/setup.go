package log

import (
	"io"
	"log"
	"os"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/response"

	"github.com/hashicorp/go-syslog"
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
			var err error
			var writer io.Writer

			if rules[i].OutputFile == "stdout" {
				writer = os.Stdout
			} else if rules[i].OutputFile == "stderr" {
				writer = os.Stderr
			} else if rules[i].OutputFile == "syslog" {
				writer, err = gsyslog.NewLogger(gsyslog.LOG_INFO, "LOCAL0", "coredns")
				if err != nil {
					return middleware.Error("log", err)
				}
			} else {
				var file *os.File
				file, err = os.OpenFile(rules[i].OutputFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
				if err != nil {
					return middleware.Error("log", err)
				}
				writer = file
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
			// Only an output file specified
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
