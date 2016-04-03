package middleware

import (
	"net"
	"strings"

	"github.com/miekg/dns"
)

// Host represents a host from the Corefile, may contain port.
type (
	Host string
	Addr string
)

// Standard will return the host portion of host, stripping
// of any port. The host will also be fully qualified and lowercased.
func (h Host) Standard() string {
	// separate host and port
	host, _, err := net.SplitHostPort(string(h))
	if err != nil {
		host, _, _ = net.SplitHostPort(string(h) + ":")
	}
	return strings.ToLower(dns.Fqdn(host))
}

// Standard will return a normalized address, if not port is specified
// port 53 is added, otherwise the port will be left as is.
func (a Addr) Standard() string {
	// separate host and port
	addr, port, err := net.SplitHostPort(string(a))
	if err != nil {
		addr, port, _ = net.SplitHostPort(string(a) + ":53")
	}
	return net.JoinHostPort(addr, port)
}
