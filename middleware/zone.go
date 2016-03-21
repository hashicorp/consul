package middleware

import "strings"

type Zones []string

// Matches checks to see if other matches p.  The match will return the most
// specific zones that matches other. The empty string signals a not found
// condition.
func (z Zones) Matches(qname string) string {
	zone := ""
	// TODO(miek): use IsSubDomain here?
	for _, zname := range z {
		if strings.HasSuffix(qname, zname) {
			if len(zname) > len(zone) {
				zone = zname
			}
		}
	}
	return zone
}
