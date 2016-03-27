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
	// TODO(miek): implement DNSSEC
	var rr dns.RR
	mk, known := dns.TypeToRR[qtype]
	if !known {
		return nil, nil, NameError
		// Uhm...?
		// rr = new(RFC3597)
	} else {
		rr = mk()
	}
	if qtype == dns.TypeSOA {
		return z.lookupSOA(do)
	}

	rr.Header().Name = qname
	elem := z.Tree.Get(rr)
	if elem == nil {
		return []dns.RR{z.SOA}, nil, NameError
	}
	rrs := elem.Types(dns.TypeCNAME)
	if len(rrs) > 0 { // should only ever be 1 actually; TODO(miek) check for this?
		// lookup target from the cname
		rr.Header().Name = rrs[0].(*dns.CNAME).Target
		elem := z.Tree.Get(rr)
		if elem == nil {
			return rrs, nil, Success
		}
		return rrs, elem.All(), Success
	}

	rrs = elem.Types(qtype)
	if len(rrs) == 0 {
		return []dns.RR{z.SOA}, nil, NoData
	}
	// Need to check sub-type on RRSIG records to only include the correctly
	// typed ones.
	return rrs, nil, Success
}

func (z *Zone) lookupSOA(do bool) ([]dns.RR, []dns.RR, Result) {
	return []dns.RR{z.SOA}, nil, Success
}

// signatureForSubType range through the signature and return the correct
// ones for the subtype.
func (z *Zone) signatureForSubType(rrs []dns.RR, subtype uint16, do bool) []dns.RR {
	if !do {
		return nil
	}
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
