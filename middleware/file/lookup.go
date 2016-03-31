package file

import (
	"github.com/miekg/coredns/middleware/file/tree"

	"github.com/miekg/dns"
)

// Result is the result of a Lookup
type Result int

const (
	Success Result = iota
	NameError
	NoData
	ServerFailure
)

// Lookup looks up qname and qtype in the zone, when do is true DNSSEC are included as well.
// Three sets of records are returned, one for the answer, one for authority  and one for the additional section.
func (z *Zone) Lookup(qname string, qtype uint16, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	var rr dns.RR
	mk, known := dns.TypeToRR[qtype]
	if !known {
		return nil, nil, nil, ServerFailure
	} else {
		rr = mk()
	}
	if qtype == dns.TypeSOA {
		return z.lookupSOA(do)
	}

	// Misuse rr to be a question.
	rr.Header().Rrtype = qtype
	rr.Header().Name = qname

	elem := z.Tree.Get(rr)
	if elem == nil {
		if elem == nil {
			return z.nameError(rr, do)
		}
	}

	rrs := elem.Types(dns.TypeCNAME)
	if len(rrs) > 0 { // should only ever be 1 actually; TODO(miek) check for this?
		rr.Header().Name = rrs[0].(*dns.CNAME).Target
		return z.lookupCNAME(rrs, rr, do)
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

func (z *Zone) nameError(rr dns.RR, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	// Is there a wildcard?
	rr1 := dns.Copy(rr)
	rr1.Header().Name = rr.Header().Name
	rr1.Header().Rrtype = rr.Header().Rrtype
	ce := z.ClosestEncloser(rr1)
	rr1.Header().Name = "*." + ce
	elem := z.Tree.Get(rr1)

	if elem != nil {
		ret := elem.Types(rr1.Header().Rrtype) // there can only be one of these (or zero)
		switch {
		case ret != nil:
			if do {
				sigs := elem.Types(dns.TypeRRSIG)
				sigs = signatureForSubType(sigs, rr.Header().Rrtype)
				ret = append(ret, sigs...)
			}
			ret = wildcardReplace(rr, ce, ret)
			return ret, nil, nil, Success
		case ret == nil:
			// nodata, nsec from the wildcard - type does not exist
			// nsec proof that name does not exist
			// TODO(miek)
		}
	}

	// name error
	ret := []dns.RR{z.SOA}
	if do {
		ret = append(ret, z.SIG...)
		ret = append(ret, z.nameErrorProof(rr)...)
	}
	return nil, ret, nil, NameError
}

func (z *Zone) lookupSOA(do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	if do {
		ret := append([]dns.RR{z.SOA}, z.SIG...)
		return ret, nil, nil, Success
	}
	return []dns.RR{z.SOA}, nil, nil, Success
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

func (z *Zone) lookupCNAME(rrs []dns.RR, rr dns.RR, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	elem := z.Tree.Get(rr)
	if elem == nil {
		return rrs, nil, nil, Success
	}
	extra := cnameForType(elem.All(), rr.Header().Rrtype)
	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, rr.Header().Rrtype)
		if len(sigs) > 0 {
			extra = append(extra, sigs...)
		}
	}
	return rrs, nil, extra, Success
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

// wildcardReplace replaces the first wildcard with label.
func wildcardReplace(rr dns.RR, ce string, rrs []dns.RR) []dns.RR {
	// Get how many labels the ce is off from the fullname, this is how much of the
	// original rr's '*' we must replace.
	labels := dns.CountLabel(rr.Header().Name) - dns.CountLabel(ce) // can not be 0, TODO(miek): check

	indexes := dns.Split(rr.Header().Name)
	if labels >= len(indexes) {
		// TODO(miek): yes then what?
		// Is the == right here?
		return nil
	}
	replacement := rr.Header().Name[:indexes[labels]]

	// need to copy here, otherwise we change in zone stuff
	ret := make([]dns.RR, len(rrs))
	for i, r := range rrs {
		ret[i] = dns.Copy(r)
		ret[i].Header().Name = replacement + r.Header().Name[2:]
	}
	return ret
}
