package dnsserver

import (
	"crypto/tls"
	"fmt"

	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

// Config configuration for a single server.
type Config struct {
	// The zone of the site.
	Zone string

	// one or several hostnames to bind the server to.
	// defaults to a single empty string that denote the wildcard address
	ListenHosts []string

	// The port to listen on.
	Port string

	// Root points to a base directory we we find user defined "things".
	// First consumer is the file plugin to looks for zone files in this place.
	Root string

	// Debug controls the panic/recover mechanism that is enabled by default.
	Debug bool

	// The transport we implement, normally just "dns" over TCP/UDP, but could be
	// DNS-over-TLS or DNS-over-gRPC.
	Transport string

	// If this function is not nil it will be used to further filter access
	// to this handler. The primary use is to limit access to a reverse zone
	// on a non-octet boundary, i.e. /17
	FilterFunc func(string) bool

	// TLSConfig when listening for encrypted connections (gRPC, DNS-over-TLS).
	TLSConfig *tls.Config

	// Plugin stack.
	Plugin []plugin.Plugin

	// Compiled plugin stack.
	pluginChain plugin.Handler

	// Plugin interested in announcing that they exist, so other plugin can call methods
	// on them should register themselves here. The name should be the name as return by the
	// Handler's Name method.
	registry map[string]plugin.Handler
}

// keyForConfig build a key for identifying the configs during setup time
func keyForConfig(blocIndex int, blocKeyIndex int) string {
	return fmt.Sprintf("%d:%d", blocIndex, blocKeyIndex)
}

// GetConfig gets the Config that corresponds to c.
// If none exist nil is returned.
func GetConfig(c *caddy.Controller) *Config {
	ctx := c.Context().(*dnsContext)
	key := keyForConfig(c.ServerBlockIndex, c.ServerBlockKeyIndex)
	if cfg, ok := ctx.keysToConfigs[key]; ok {
		return cfg
	}
	// we should only get here during tests because directive
	// actions typically skip the server blocks where we make
	// the configs.
	ctx.saveConfig(key, &Config{ListenHosts: []string{""}})
	return GetConfig(c)
}
