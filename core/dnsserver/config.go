package dnsserver

import (
	"crypto/tls"

	"github.com/coredns/coredns/middleware"

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

	// Debug controls the panic/recover mechanism that is enabled by default.
	Debug bool

	// The transport we implement, normally just "dns" over TCP/UDP, but could be
	// DNS-over-TLS or DNS-over-gRPC.
	Transport string

	// TLSConfig when listening for encrypted connections (gRPC, DNS-over-TLS).
	TLSConfig *tls.Config

	// Middleware stack.
	Middleware []middleware.Middleware

	// Compiled middleware stack.
	middlewareChain middleware.Handler

	// Middleware interested in announcing that they exist, so other middleware can call methods
	// on them should register themselves here. The name should be the name as return by the
	// Handler's Name method.
	Registry map[string]middleware.Handler
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
