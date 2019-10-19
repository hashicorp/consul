package plugin

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/miekg/dns"
)

// See core/dnsserver/address.go - we should unify these two impls.

// Zones represents a lists of zone names.
type Zones []string

// Matches checks if qname is a subdomain of any of the zones in z.  The match
// will return the most specific zones that matches. The empty string
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

// Normalize fully qualifies all zones in z. The zones in Z must be domain names, without
// a port or protocol prefix.
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
	Host string
)

// Normalize will return the host portion of host, stripping
// of any port or transport. The host will also be fully qualified and lowercased.
// An empty string is returned on failure
func (h Host) Normalize() string {
	// The error can be ignored here, because this function should only be called after the corefile has already been vetted.
	host, _ := h.MustNormalize()
	return host
}

// MustNormalize will return the host portion of host, stripping
// of any port or transport. The host will also be fully qualified and lowercased.
// An error is returned on error
func (h Host) MustNormalize() (string, error) {
	s := string(h)
	_, s = parse.Transport(s)

	// The error can be ignored here, because this function is called after the corefile has already been vetted.
	host, _, _, err := SplitHostPort(s)
	if err != nil {
		return "", err
	}
	return Name(host).Normalize(), nil
}

// SplitHostPort splits s up in a host and port portion, taking reverse address notation into account.
// String the string s should *not* be prefixed with any protocols, i.e. dns://. The returned ipnet is the
// *net.IPNet that is used when the zone is a reverse and a netmask is given.
func SplitHostPort(s string) (host, port string, ipnet *net.IPNet, err error) {
	// If there is: :[0-9]+ on the end we assume this is the port. This works for (ascii) domain
	// names and our reverse syntax, which always needs a /mask *before* the port.
	// So from the back, find first colon, and then check if it's a number.
	host = s

	colon := strings.LastIndex(s, ":")
	if colon == len(s)-1 {
		return "", "", nil, fmt.Errorf("expecting data after last colon: %q", s)
	}
	if colon != -1 {
		if p, err := strconv.Atoi(s[colon+1:]); err == nil {
			port = strconv.Itoa(p)
			host = s[:colon]
		}
	}

	// TODO(miek): this should take escaping into account.
	if len(host) > 255 {
		return "", "", nil, fmt.Errorf("specified zone is too long: %d > 255", len(host))
	}

	_, d := dns.IsDomainName(host)
	if !d {
		return "", "", nil, fmt.Errorf("zone is not a valid domain name: %s", host)
	}

	// Check if it parses as a reverse zone, if so we use that. Must be fully specified IP and mask.
	ip, n, err := net.ParseCIDR(host)
	ones, bits := 0, 0
	if err == nil {
		if rev, e := dns.ReverseAddr(ip.String()); e == nil {
			ones, bits = n.Mask.Size()
			// get the size, in bits, of each portion of hostname defined in the reverse address. (8 for IPv4, 4 for IPv6)
			sizeDigit := 8
			if len(n.IP) == net.IPv6len {
				sizeDigit = 4
			}
			// Get the first lower octet boundary to see what encompassing zone we should be authoritative for.
			mod := (bits - ones) % sizeDigit
			nearest := (bits - ones) + mod
			offset := 0
			var end bool
			for i := 0; i < nearest/sizeDigit; i++ {
				offset, end = dns.NextLabel(rev, offset)
				if end {
					break
				}
			}
			host = rev[offset:]
		}
	}
	return host, port, n, nil
}
