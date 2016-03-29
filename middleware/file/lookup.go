package file

import (
	"fmt"

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
		// wildcard lookup
		return z.nameError(elem, rr, do)
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
		if len(sigs) > 0 {
			rrs = append(rrs, sigs...)
		}
	}
	return rrs, nil, nil, Success
}

func (z *Zone) noData(elem *tree.Elem, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	soa, _, _, _ := z.lookupSOA(do)
	nsec := z.lookupNSEC(elem, do)
	return nil, append(soa, nsec...), nil, Success
}

func (z *Zone) nameError(elem *tree.Elem, rr dns.RR, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	ret := []dns.RR{}
	if do {
		ret = append(ret, z.SIG...)
		// Now we need two NSEC, one to deny the wildcard and one to deny the name.
		elem := z.Tree.Prev(rr)
		fmt.Printf("%+v\n", elem.All())
		elem = z.Tree.Prev(wildcard(rr))
		fmt.Printf("%+v\n", elem.All())
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

// wildcard returns rr with the first label exchanged for a wildcard '*'.
func wildcard(rr dns.RR) dns.RR {
	// root label, TODO(miek)
	s := rr.Header().Name
	i, _ := dns.NextLabel(s, 0)
	rr.Header().Name = "*" + s[i:]
	return rr
}
