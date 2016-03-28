package file

import "github.com/miekg/dns"

// Result is the result of a Lookup
type Result int

const (
	Success Result = iota
	NameError
	NoData // aint no offical NoData return code.
)

// Lookup looks up qname and qtype in the zone, when do is true DNSSEC are included as well.
// Two sets of records are returned, one for the answer and one for the additional section.
func (z *Zone) Lookup(qname string, qtype uint16, do bool) ([]dns.RR, []dns.RR, Result) {
	var rr dns.RR
	mk, known := dns.TypeToRR[qtype]
	if !known {
		an, ad, _ := z.lookupSOA(do)
		return an, ad, NameError
		// Uhm...? rr = new(RFC3597) ??
	} else {
		rr = mk()
	}
	rr.Header().Rrtype = qtype // this is pretty nonobvious

	if qtype == dns.TypeSOA {
		return z.lookupSOA(do)
	}

	rr.Header().Name = qname
	elem := z.Tree.Get(rr)
	if elem == nil {
		an, ad, _ := z.lookupSOA(do)
		return an, ad, NameError
	}

	rrs := elem.Types(dns.TypeCNAME)
	if len(rrs) > 0 { // should only ever be 1 actually; TODO(miek) check for this?
		rr.Header().Name = rrs[0].(*dns.CNAME).Target
		return z.lookupCNAME(rrs, rr, do)
	}

	rrs = elem.Types(qtype)
	if len(rrs) == 0 {
		an, ad, _ := z.lookupSOA(do)
		return an, ad, NoData
	}
	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, qtype)
		if len(sigs) > 0 {
			rrs = append(rrs, sigs...)
		}
	}
	return rrs, nil, Success
}

func (z *Zone) lookupSOA(do bool) ([]dns.RR, []dns.RR, Result) {
	if do {
		ret := append([]dns.RR{z.SOA}, z.SIG...)
		return ret, nil, Success
	}
	return []dns.RR{z.SOA}, nil, Success
}

func (z *Zone) lookupCNAME(rrs []dns.RR, rr dns.RR, do bool) ([]dns.RR, []dns.RR, Result) {
	elem := z.Tree.Get(rr)
	if elem == nil {
		return rrs, nil, Success
	}
	extra := cnameForType(elem.All(), rr.Header().Rrtype)
	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, rr.Header().Rrtype)
		if len(sigs) > 0 {
			extra = append(extra, sigs...)
		}
	}
	return rrs, extra, Success
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
