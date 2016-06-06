package kubernetes

import (
	"strings"

	"github.com/miekg/dns"
)

// Domain is the opposite of Path.
func (k Kubernetes) Domain(s string) string {
	l := strings.Split(s, "/")
	// start with 1, to strip /skydns
	for i, j := 1, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
	return dns.Fqdn(strings.Join(l[1:len(l)-1], "."))
}
