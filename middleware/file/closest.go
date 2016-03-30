package file

import "github.com/miekg/dns"

// ClosestEncloser returns the closest encloser for rr.
func (z *Zone) ClosestEncloser(rr dns.RR) string {
	elem := z.Tree.Prev(rr)
	if elem == nil {
		// SOA?
		return ""
	}
	for _, r := range elem.All() {
		return r.Header().Name
	}
	return ""
}
