package etcd

import (
	"path"
	"strings"

	"github.com/miekg/dns"
)

// Path converts a domainname to an etcd path. If s looks like service.staging.skydns.local.,
// the resulting key will be /skydns/local/skydns/staging/service .
func (e Etcd) Path(s string) string {
	l := dns.SplitDomainName(s)
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
	return path.Join(append([]string{"/" + e.PathPrefix + "/"}, l...)...)
}

// Domain is the opposite of Path.
func (e Etcd) Domain(s string) string {
	l := strings.Split(s, "/")
	// start with 1, to strip /skydns
	for i, j := 1, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
	return dns.Fqdn(strings.Join(l[1:len(l)-1], "."))
}

// As Path, but if a name contains wildcards (* or any), the name will be
// chopped of before the (first) wildcard, and we do a highler evel search and
// later find the matching names.  So service.*.skydns.local, will look for all
// services under skydns.local and will later check for names that match
// service.*.skydns.local.  If a wildcard is found the returned bool is true.
func (e Etcd) PathWithWildcard(s string) (string, bool) {
	l := dns.SplitDomainName(s)
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
	for i, k := range l {
		if k == "*" || k == "any" {
			return path.Join(append([]string{"/" + e.PathPrefix + "/"}, l[:i]...)...), true
		}
	}
	return path.Join(append([]string{"/" + e.PathPrefix + "/"}, l...)...), false
}
