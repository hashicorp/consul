package middleware

import (
	"strings"

	"github.com/miekg/dns"
)

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
func (n Name) Normalize() string {
	return strings.ToLower(dns.Fqdn(string(n)))
}
