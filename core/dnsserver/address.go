package dnsserver

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type zoneAddr struct {
	Zone string
	Port string
}

// String return z.Zone + ":" + z.Port as a string.
func (z zoneAddr) String() string { return z.Zone + ":" + z.Port }

// normalizeZone parses an zone string into a structured format with separate
// host, and port portions, as well as the original input string.
//
// TODO(miek): possibly move this to middleware/normalize.go
func normalizeZone(str string) (zoneAddr, error) {
	var err error

	// separate host and port
	host, port, err := net.SplitHostPort(str)
	if err != nil {
		host, port, err = net.SplitHostPort(str + ":")
		// no error check here; return err at end of function
	}

	if len(host) > 255 {
		return zoneAddr{}, fmt.Errorf("specified zone is too long: %d > 255", len(host))
	}
	_, d := dns.IsDomainName(host)
	if !d {
		return zoneAddr{}, fmt.Errorf("zone is not a valid domain name: %s", host)
	}

	if port == "" {
		port = Port
	}

	return zoneAddr{Zone: strings.ToLower(dns.Fqdn(host)), Port: port}, err
}
