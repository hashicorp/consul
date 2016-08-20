package log

import (
	"io"
	"log"
	"os"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/server"

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
		return err
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
					return err
				}
			} else {
				var file *os.File
				file, err = os.OpenFile(rules[i].OutputFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
				if err != nil {
					return err
				}
				if rules[i].Roller != nil {
					file.Close()
					rules[i].Roller.Filename = rules[i].OutputFile
					writer = rules[i].Roller.GetLogWriter()
				} else {
					writer = file
				}
			}

			rules[i].Log = log.New(writer, "", 0)
		}

		return nil
	})

	dnsserver.GetConfig(c).AddMiddleware(func(next dnsserver.Handler) dnsserver.Handler {
		return Logger{Next: next, Rules: rules, ErrorFunc: server.DefaultErrorFunc}
	})

	return nil
}

func logParse(c *caddy.Controller) ([]Rule, error) {
	var rules []Rule

	for c.Next() {
		args := c.RemainingArgs()

		var logRoller *middleware.LogRoller
		if c.NextBlock() {
			if c.Val() == "rotate" {
				if c.NextArg() {
					if c.Val() == "{" {
						var err error
						logRoller, err = middleware.ParseRoller(c)
						if err != nil {
							return nil, err
						}
						// This part doesn't allow having something after the rotate block
						if c.Next() {
							if c.Val() != "}" {
								return nil, c.ArgErr()
							}
						}
					}
				}
			}
		}
		if len(args) == 0 {
			// Nothing specified; use defaults
			rules = append(rules, Rule{
				NameScope:  ".",
				OutputFile: DefaultLogFilename,
				Format:     DefaultLogFormat,
				Roller:     logRoller,
			})
		} else if len(args) == 1 {
			// Only an output file specified
			rules = append(rules, Rule{
				NameScope:  ".",
				OutputFile: args[0],
				Format:     DefaultLogFormat,
				Roller:     logRoller,
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
				Roller:     logRoller,
			})
		}
	}

	return rules, nil
}
