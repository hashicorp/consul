package errors

import (
	"io"
	"log"
	"os"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"

	"github.com/hashicorp/go-syslog"
	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("errors", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	handler, err := errorsParse(c)
	if err != nil {
		return middleware.Error("errors", err)
	}

	var writer io.Writer

	switch handler.LogFile {
	case "visible":
		handler.Debug = true
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	case "syslog":
		writer, err = gsyslog.NewLogger(gsyslog.LOG_ERR, "LOCAL0", "coredns")
		if err != nil {
			return middleware.Error("errors", err)
		}
	default:
		if handler.LogFile == "" {
			writer = os.Stderr // default
			break
		}

		var file *os.File
		file, err = os.OpenFile(handler.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return middleware.Error("errors", err)
		}
		writer = file
	}
	handler.Log = log.New(writer, "", 0)

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		handler.Next = next
		return handler
	})

	return nil
}

func errorsParse(c *caddy.Controller) (errorHandler, error) {
	handler := errorHandler{}

	optionalBlock := func() (bool, error) {
		var hadBlock bool

		for c.NextBlock() {
			hadBlock = true

			what := c.Val()
			if !c.NextArg() {
				return hadBlock, c.ArgErr()
			}
			where := c.Val()

			if what == "log" {
				if where == "visible" {
					handler.Debug = true
				} else {
					handler.LogFile = where
				}
			}
		}
		return hadBlock, nil
	}

	for c.Next() {
		// Configuration may be in a block
		hadBlock, err := optionalBlock()
		if err != nil {
			return handler, err
		}

		// Otherwise, the only argument would be an error log file name or 'visible'
		if !hadBlock {
			if c.NextArg() {
				if c.Val() == "visible" {
					handler.Debug = true
				} else {
					handler.LogFile = c.Val()
				}
			}
		}
	}

	return handler, nil
}
