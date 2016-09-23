package file

import (
	"github.com/miekg/coredns/middleware/file/tree"

	"github.com/miekg/dns"
)

// Result is the result of a Lookup
type Result int

const (
	// Success is a successful lookup.
	Success Result = iota
	// NameError indicates a nameerror
	NameError
	// Delegation indicates the lookup resulted in a delegation.
	Delegation
	// NoData indicates the lookup resulted in a NODATA.
	NoData
	// ServerFailure indicates a server failure during the lookup.
	ServerFailure
)

// Lookup looks up qname and qtype in the zone. When do is true DNSSEC records are included.
// Three sets of records are returned, one for the answer, one for authority  and one for the additional section.
func (z *Zone) Lookup(qname string, qtype uint16, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	if qtype == dns.TypeSOA {
		return z.lookupSOA(do)
	}
	if qtype == dns.TypeNS && qname == z.origin {
		return z.lookupNS(do)
	}

	elem, res := z.Tree.Search(qname, qtype)
	if elem == nil {
		if res == tree.EmptyNonTerminal {
			return z.emptyNonTerminal(qname, do)
		}
		return z.nameError(qname, qtype, do)
	}
	if res == tree.Delegation {
		rrs := elem.Types(dns.TypeNS)
		glue := []dns.RR{}
		for _, ns := range rrs {
			if dns.IsSubDomain(ns.Header().Name, ns.(*dns.NS).Ns) {
				// even with Do, this should be unsigned.
				elem, res := z.Tree.SearchGlue(ns.(*dns.NS).Ns)
				if res == tree.Found {
					glue = append(glue, elem.Types(dns.TypeAAAA)...)
					glue = append(glue, elem.Types(dns.TypeA)...)
				}
			}
		}
		return nil, rrs, glue, Delegation
	}

	rrs := elem.Types(dns.TypeCNAME)
	if len(rrs) > 0 { // should only ever be 1 actually; TODO(miek) check for this?
		return z.lookupCNAME(rrs, qtype, do)
	}

	rrs = elem.Types(qtype)
	if len(rrs) == 0 {
		return z.noData(elem, do)
	}

	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, qtype)
		rrs = append(rrs, sigs...)
	}
	return rrs, nil, nil, Success
}

func (z *Zone) noData(elem *tree.Elem, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	soa, _, _, _ := z.lookupSOA(do)
	nsec := z.lookupNSEC(elem, do)
	return nil, append(soa, nsec...), nil, Success
}

func (z *Zone) emptyNonTerminal(qname string, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	soa, _, _, _ := z.lookupSOA(do)

	elem := z.Tree.Prev(qname)
	nsec := z.lookupNSEC(elem, do)
	return nil, append(soa, nsec...), nil, Success
}

func (z *Zone) nameError(qname string, qtype uint16, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	// Is there a wildcard?
	ce := z.ClosestEncloser(qname, qtype)
	elem, _ := z.Tree.Search("*."+ce, qtype) // use result here?

	if elem != nil {
		ret := elem.Types(qtype) // there can only be one of these (or zero)
		switch {
		case ret != nil:
			if do {
				sigs := elem.Types(dns.TypeRRSIG)
				sigs = signatureForSubType(sigs, qtype)
				ret = append(ret, sigs...)
			}
			ret = wildcardReplace(qname, ce, ret)
			return ret, nil, nil, Success
		case ret == nil:
			// nodata, nsec from the wildcard - type does not exist
			// nsec proof that name does not exist
			// TODO(miek)
		}
	}

	// name error
	ret := []dns.RR{z.Apex.SOA}
	if do {
		ret = append(ret, z.Apex.SIGSOA...)
		ret = append(ret, z.nameErrorProof(qname, qtype)...)
	}
	return nil, ret, nil, NameError
}

func (z *Zone) lookupSOA(do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	if do {
		ret := append([]dns.RR{z.Apex.SOA}, z.Apex.SIGSOA...)
		return ret, nil, nil, Success
	}
	return []dns.RR{z.Apex.SOA}, nil, nil, Success
}

func (z *Zone) lookupNS(do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	if do {
		ret := append(z.Apex.NS, z.Apex.SIGNS...)
		return ret, nil, nil, Success
	}
	return z.Apex.NS, nil, nil, Success
}

// lookupNSEC looks up nsec and sigs.
func (z *Zone) lookupNSEC(elem *tree.Elem, do bool) []dns.RR {
	if !do {
		return nil
	}
	nsec := elem.Types(dns.TypeNSEC)
	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, dns.TypeNSEC)
		if len(sigs) > 0 {
			nsec = append(nsec, sigs...)
		}
	}
	return nsec
}

func (z *Zone) lookupCNAME(rrs []dns.RR, qtype uint16, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	elem, _ := z.Tree.Search(rrs[0].(*dns.CNAME).Target, qtype)
	if elem == nil {
		return rrs, nil, nil, Success
	}
	targets := cnameForType(elem.All(), qtype)
	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, qtype)
		if len(sigs) > 0 {
			targets = append(targets, sigs...)
		}
	}
	return append(rrs, targets...), nil, nil, Success
}

func cnameForType(targets []dns.RR, origQtype uint16) []dns.RR {
	ret := []dns.RR{}
	for _, target := range targets {
		if target.Header().Rrtype == origQtype {
			ret = append(ret, target)
		}
	}
	return ret
}

// signatureForSubType range through the signature and return the correct
// ones for the subtype.
func signatureForSubType(rrs []dns.RR, subtype uint16) []dns.RR {
	sigs := []dns.RR{}
	for _, sig := range rrs {
		if s, ok := sig.(*dns.RRSIG); ok {
			if s.TypeCovered == subtype {
				sigs = append(sigs, s)
			}
		}
	}
	return sigs
}

// wildcardReplace replaces the ownername with the original query name.
func wildcardReplace(qname, ce string, rrs []dns.RR) []dns.RR {
	// need to copy here, otherwise we change in zone stuff
	ret := make([]dns.RR, len(rrs))
	for i, r := range rrs {
		ret[i] = dns.Copy(r)
		ret[i].Header().Name = qname
	}
	return ret
}
