package file

import "github.com/miekg/dns"

// ClosestEncloser returns the closest encloser for rr.
func (z *Zone) ClosestEncloser(rr dns.RR) string {
	// tree/tree.go does not store a parent *Node pointer, so we can't
	// just follow up the tree. TODO(miek): fix.
	offset, end := dns.NextLabel(rr.Header().Name, 0)
	for !end {
		elem, _ := z.Tree.Get(rr)
		if elem != nil {
			return elem.Name()
		}
		rr.Header().Name = rr.Header().Name[offset:]

		offset, end = dns.NextLabel(rr.Header().Name, offset)
	}

	return z.SOA.Header().Name
}

// nameErrorProof finds the closest encloser and return an NSEC that proofs
// the wildcard does not exist and an NSEC that proofs the name does no exist.
func (z *Zone) nameErrorProof(rr dns.RR) []dns.RR {
	elem := z.Tree.Prev(rr)
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
	ce := z.ClosestEncloser(rr)
	wildcard := "*." + ce
	rr.Header().Name = wildcard
	elem = z.Tree.Prev(rr)
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
