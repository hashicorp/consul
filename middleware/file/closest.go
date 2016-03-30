package file

import "github.com/miekg/dns"

// ClosestEncloser returns the closest encloser for rr.
func (z *Zone) ClosestEncloser(rr dns.RR) string {
	// tree/tree.go does not store a parent *Node pointer, so we can't
	// just follow up the tree. TODO(miek): fix.

	offset, end := dns.NextLabel(rr.Header().Name, 0)
	for !end {
		elem := z.Tree.Get(rr)
		if elem != nil {
			return elem.Name()
		}
		rr.Header().Name = rr.Header().Name[offset:]

		offset, end = dns.NextLabel(rr.Header().Name, offset)
	}

	return z.SOA.Header().Name
}
