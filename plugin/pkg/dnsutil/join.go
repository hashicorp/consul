package dnsutil

import (
	"strings"

	"github.com/miekg/dns"
)

// Join joins labels to form a fully qualified domain name. If the last label is
// the root label it is ignored. Not other syntax checks are performed.
func Join(labels ...string) string {
	ll := len(labels)
	if labels[ll-1] == "." {
		return strings.Join(labels[:ll-1], ".") + "."
	}
	return dns.Fqdn(strings.Join(labels, "."))
}
