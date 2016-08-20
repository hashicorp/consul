package dnsserver

import "github.com/mholt/caddy"

// Config configuration for a single server.
type Config struct {
	// The zone of the site.
	Zone string

	// The hostname to bind listener to, defaults to the wildcard address
	ListenHost string

	// The port to listen on.
	Port string

	// The directory from which to parse db files, and store keys.
	Root string

	// Middleware stack.
	Middleware []Middleware

	// Compiled middleware stack.
	middlewareChain Handler
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
	ctx.saveConfig(c.Key, &Config{Root: Root})
	return GetConfig(c)
}
