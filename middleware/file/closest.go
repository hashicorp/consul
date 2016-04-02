package file

import "github.com/miekg/dns"

// ClosestEncloser returns the closest encloser for rr.
func (z *Zone) ClosestEncloser(qname string, qtype uint16) string {
	// tree/tree.go does not store a parent *Node pointer, so we can't
	// just follow up the tree. TODO(miek): fix.
	offset, end := dns.NextLabel(qname, 0)
	for !end {
		elem, _ := z.Tree.Search(qname, qtype)
		if elem != nil {
			return elem.Name()
		}
		qname = qname[offset:]

		offset, end = dns.NextLabel(qname, offset)
	}

	return z.SOA.Header().Name
}

// nameErrorProof finds the closest encloser and return an NSEC that proofs
// the wildcard does not exist and an NSEC that proofs the name does no exist.
func (z *Zone) nameErrorProof(qname string, qtype uint16) []dns.RR {
	elem := z.Tree.Prev(qname)
	if elem == nil {
		return nil
	}
	nsec := z.lookupNSEC(elem, true)
	nsecIndex := 0
	for i := 0; i < len(nsec); i++ {
		if nsec[i].Header().Rrtype == dns.TypeNSEC {
			nsecIndex = i
			break
		}
	}

	// We do this lookup twice, once for wildcard and once for the name proof. TODO(miek): fix
	ce := z.ClosestEncloser(qname, qtype)
	elem = z.Tree.Prev("*." + ce)
	if elem == nil {
		// Root?
		return nil
	}
	nsec1 := z.lookupNSEC(elem, true)
	nsec1Index := 0
	for i := 0; i < len(nsec1); i++ {
		if nsec1[i].Header().Rrtype == dns.TypeNSEC {
			nsec1Index = i
			break
		}
	}

	// Check for duplicate NSEC.
	if nsec[nsecIndex].Header().Name == nsec1[nsec1Index].Header().Name &&
		nsec[nsecIndex].(*dns.NSEC).NextDomain == nsec1[nsec1Index].(*dns.NSEC).NextDomain {
		return nsec
	}

	return append(nsec, nsec1...)
}
