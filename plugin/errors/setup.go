package errors

import (
	"regexp"
	"time"

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

	c.OnShutdown(func() error {
		handler.stop()
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		handler.Next = next
		return handler
	})

	return nil
}

func errorsParse(c *caddy.Controller) (*errorHandler, error) {
	handler := newErrorHandler()

	i := 0
	for c.Next() {
		if i > 0 {
			return nil, plugin.ErrOnce
		}
		i++

		args := c.RemainingArgs()
		switch len(args) {
		case 0:
		case 1:
			if args[0] != "stdout" {
				return nil, c.Errf("invalid log file: %s", args[0])
			}
		default:
			return nil, c.ArgErr()
		}

		for c.NextBlock() {
			if err := parseBlock(c, handler); err != nil {
				return nil, err
			}
		}
	}
	return handler, nil
}

func parseBlock(c *caddy.Controller, h *errorHandler) error {
	if c.Val() != "consolidate" {
		return c.SyntaxErr("consolidate")
	}

	args := c.RemainingArgs()
	if len(args) != 2 {
		return c.ArgErr()
	}
	p, err := time.ParseDuration(args[0])
	if err != nil {
		return c.Err(err.Error())
	}
	re, err := regexp.Compile(args[1])
	if err != nil {
		return c.Err(err.Error())
	}
	h.patterns = append(h.patterns, &pattern{period: p, pattern: re})

	return nil
}
