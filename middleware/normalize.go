package middleware

import (
	"net"
	"strings"

	"github.com/miekg/dns"
)

// Zones respresents a lists of zone names.
type Zones []string

// Matches checks is qname is a subdomain of any of the zones in z.  The match
// will return the most specific zones that matches other. The empty string
// signals a not found condition.
func (z Zones) Matches(qname string) string {
	zone := ""
	for _, zname := range z {
		if dns.IsSubDomain(zname, qname) {
			// We want the *longest* matching zone, otherwise we may end up in a parent
			if len(zname) > len(zone) {
				zone = zname
			}
		}
	}
	return zone
}

// Normalize fully qualifies all zones in z.
func (z Zones) Normalize() {
	for i := range z {
		z[i] = Name(z[i]).Normalize()
	}
}

// Name represents a domain name.
type Name string

// Matches checks to see if other is a subdomain (or the same domain) of n.
// This method assures that names can be easily and consistently matched.
func (n Name) Matches(child string) bool {
	if dns.Name(n) == dns.Name(child) {
		return true
	}
	return dns.IsSubDomain(string(n), child)
}

// Normalize lowercases and makes n fully qualified.
func (n Name) Normalize() string { return strings.ToLower(dns.Fqdn(string(n))) }

type (
	// Host represents a host from the Corefile, may contain port.
	Host string // Host represents a host from the Corefile, may contain port.
	// Addr represents an address in the Corefile.
	Addr string // Addr resprents an address in the Corefile.
)

// Normalize will return the host portion of host, stripping
// of any port. The host will also be fully qualified and lowercased.
func (h Host) Normalize() string {
	// separate host and port
	host, _, err := net.SplitHostPort(string(h))
	if err != nil {
		host, _, _ = net.SplitHostPort(string(h) + ":")
	}
	return Name(host).Normalize()
}

// Normalize will return a normalized address, if not port is specified
// port 53 is added, otherwise the port will be left as is.
func (a Addr) Normalize() string {
	// separate host and port
	addr, port, err := net.SplitHostPort(string(a))
	if err != nil {
		addr, port, _ = net.SplitHostPort(string(a) + ":53")
	}
	// TODO(miek): lowercase it?
	return net.JoinHostPort(addr, port)
}
