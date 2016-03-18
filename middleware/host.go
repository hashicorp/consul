package middleware

import (
	"net"
	"strings"

	"github.com/miekg/dns"
)

// Host represents a host from the Caddyfile, may contain port.
type Host string

// Standard host will return the host portion of host, stripping
// of any port. The host will also be fully qualified and lowercased.
func (h Host) StandardHost() string {
	// separate host and port
	host, _, err := net.SplitHostPort(string(h))
	if err != nil {
		host, _, _ = net.SplitHostPort(string(h) + ":")
	}
	return strings.ToLower(dns.Fqdn(host))
}
