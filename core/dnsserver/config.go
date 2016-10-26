package dnsserver

import (
	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
)

// Config configuration for a single server.
type Config struct {
	// The zone of the site.
	Zone string

	// The hostname to bind listener to, defaults to the wildcard address
	ListenHost string

	// The port to listen on.
	Port string

	// Root points to a base directory we we find user defined "things".
	// First consumer is the file middleware to looks for zone files in this place.
	Root string

	// Middleware stack.
	Middleware []middleware.Middleware

	// Compiled middleware stack.
	middlewareChain middleware.Handler
}

// GetConfig gets the Config that corresponds to c.
// If none exist nil is returned.
func GetConfig(c *caddy.Controller) *Config {
	ctx := c.Context().(*dnsContext)
	if cfg, ok := ctx.keysToConfigs[c.Key]; ok {
		return cfg
	}
	// we should only get here during tests because directive
	// actions typically skip the server blocks where we make
	// the configs.
	ctx.saveConfig(c.Key, &Config{})
	return GetConfig(c)
}

// GetMiddleware returns the middleware handler that has been added to the config under name.
// This is useful to inspect if a certain middleware is active in this server.
// Note that this is order dependent and the order is defined in directives.go, i.e. if your middleware
// comes before the middleware you are checking; it will not be there (yet).
func GetMiddleware(c *caddy.Controller, name string) middleware.Handler {
	// TODO(miek): calling the handler h(nil) should be a noop...
	conf := GetConfig(c)
	for _, h := range conf.Middleware {
		x := h(nil)
		if name == x.Name() {
			return x
		}
	}
	return nil
}
