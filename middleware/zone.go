package middleware

import "github.com/miekg/dns"

type Zones []string

// Matches checks is qname is a subdomain of any of the zones in z.  The match
// will return the most specific zones that matches other. The empty string
// signals a not found condition.
func (z Zones) Matches(qname string) string {
	zone := ""
	for _, zname := range z {
		if dns.IsSubDomain(zname, qname) {
			if len(zname) > len(zone) {
				zone = zname
			}
		}
	}
	return zone
}

// FullyQualify fully qualifies all zones in z.
func (z Zones) FullyQualify() {
	for i, _ := range z {
		z[i] = dns.Fqdn(z[i])
	}
}
