package errors

import (
	"fmt"
	"log"
	"os"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

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
		return plugin.Error("errors", err)
	}

	handler.Log = log.New(os.Stdout, "", 0)

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		handler.Next = next
		return handler
	})

	return nil
}

func errorsParse(c *caddy.Controller) (errorHandler, error) {
	handler := errorHandler{}

	for c.Next() {
		args := c.RemainingArgs()
		switch len(args) {
		case 0:
			handler.LogFile = "stdout"
		case 1:
			if args[0] != "stdout" {
				return handler, fmt.Errorf("invalid log file: %s", args[0])
			}
			handler.LogFile = args[0]
		default:
			return handler, c.ArgErr()
		}
	}
	return handler, nil
}
