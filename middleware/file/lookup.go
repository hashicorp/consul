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
	if !z.NoReload {
		z.reloadMu.RLock()
	}
	defer func() {
		if !z.NoReload {
			z.reloadMu.RUnlock()
		}
	}()

	if qtype == dns.TypeSOA {
		soa := z.soa(do)
		return soa, nil, nil, Success
	}
	if qtype == dns.TypeNS && qname == z.origin {
		nsrrs := z.ns(do)
		glue := z.Glue(nsrrs)
		return nsrrs, nil, glue, Success
	}

	var (
		found, shot    bool
		parts          string
		i              int
		elem, wildElem *tree.Elem
	)

	// Lookup:
	// * Per label from the right, look if it exists. We do this to find potential
	//   delegation records.
	// * If the per-label search finds nothing, we will look for the wildcard at the
	//   level. If found we keep it around. If we don't find the complete name we will
	//   use the wildcard.
	//
	// Main for-loop handles delegation and finding or not finding the qname.
	// If found we check if it is a CNAME and do CNAME processing (DNAME should be added as well)
	// We also check if we have type and do a nodata resposne.
	//
	// If not found, we check the potential wildcard, and use that for further processing.
	// If not found and no wildcard we will process this as an NXDOMAIN response.
	//
	for {
		parts, shot = z.nameFromRight(qname, i)
		// We overshot the name, break and check if we previously found something.
		if shot {
			break
		}

		elem, found = z.Tree.Search(parts)
		if !found {
			// Apex will always be found, when we are here we can search for a wildcard
			// and save the result of that search. So when nothing match, but we have a
			// wildcard we should expand the wildcard. There can only be one wildcard,
			// so when we found one, we won't look for another.

			if wildElem == nil {
				wildcard := replaceWithAsteriskLabel(parts)
				wildElem, _ = z.Tree.Search(wildcard)
			}

			// Keep on searching, because maybe we hit an empty-non-terminal (which aren't
			// stored in the tree. Only when we have match the full qname (and possible wildcard
			// we can be confident that we didn't find anything.
			i++
			continue
		}

		// If we see NS records, it means the name as been delegated, and we should return the delegation.
		if nsrrs := elem.Types(dns.TypeNS); nsrrs != nil {
			glue := z.Glue(nsrrs)
			// If qtype == NS, we should returns success to put RRs in answer.
			if qtype == dns.TypeNS {
				return nsrrs, nil, glue, Success
			}

			if do {
				dss := z.typeFromElem(elem, dns.TypeDS, do)
				nsrrs = append(nsrrs, dss...)
			}

			return nil, nsrrs, glue, Delegation
		}

		i++
	}

	// What does found and !shot mean - do we ever hit it?
	if found && !shot {
		return nil, nil, nil, ServerFailure
	}

	// Found entire name.
	if found && shot {

		// DNAME...
		if rrs := elem.Types(dns.TypeCNAME); len(rrs) > 0 && qtype != dns.TypeCNAME {
			return z.searchCNAME(elem, rrs, qtype, do)
		}

		rrs := elem.Types(qtype, qname)

		// NODATA
		if len(rrs) == 0 {
			ret := z.soa(do)
			if do {
				nsec := z.typeFromElem(elem, dns.TypeNSEC, do)
				ret = append(ret, nsec...)
			}
			return nil, ret, nil, NoData
		}

		if do {
			sigs := elem.Types(dns.TypeRRSIG)
			sigs = signatureForSubType(sigs, qtype)
			rrs = append(rrs, sigs...)
		}

		return rrs, nil, nil, Success

	}

	// Haven't found the original name.

	// Found wildcard.
	if wildElem != nil {
		auth := []dns.RR{}

		if rrs := wildElem.Types(dns.TypeCNAME, qname); len(rrs) > 0 {
			return z.searchCNAME(wildElem, rrs, qtype, do)
		}

		rrs := wildElem.Types(qtype, qname)

		// NODATA response.
		if len(rrs) == 0 {
			ret := z.soa(do)
			if do {
				nsec := z.typeFromElem(wildElem, dns.TypeNSEC, do)
				ret = append(ret, nsec...)
			}
			return nil, ret, nil, Success
		}

		if do {
			// An NSEC is needed to say no longer name exists under this wildcard.
			if deny, found := z.Tree.Prev(qname); found {
				nsec := z.typeFromElem(deny, dns.TypeNSEC, do)
				auth = append(auth, nsec...)
			}

			sigs := wildElem.Types(dns.TypeRRSIG, qname)
			sigs = signatureForSubType(sigs, qtype)
			rrs = append(rrs, sigs...)

		}
		return rrs, auth, nil, Success
	}

	rcode := NameError

	// Hacky way to get around empty-non-terminals. If a longer name does exist, but this qname, does not, it
	// must be an empty-non-terminal. If so, we do the proper NXDOMAIN handling, but the the rcode to be success.
	if x, found := z.Tree.Next(qname); found {
		if dns.IsSubDomain(qname, x.Name()) {
			rcode = Success
		}
	}

	ret := z.soa(do)
	if do {
		deny, found := z.Tree.Prev(qname)
		nsec := z.typeFromElem(deny, dns.TypeNSEC, do)
		ret = append(ret, nsec...)

		if rcode != NameError {
			goto Out
		}

		ce, found := z.ClosestEncloser(qname)

		// wildcard denial only for NXDOMAIN
		if found {
			// wildcard denial
			wildcard := "*." + ce.Name()
			if ss, found := z.Tree.Prev(wildcard); found {
				// Only add this nsec if it is different than the one already added
				if ss.Name() != deny.Name() {
					nsec := z.typeFromElem(ss, dns.TypeNSEC, do)
					ret = append(ret, nsec...)
				}
			}
		}

	}
Out:
	return nil, ret, nil, rcode
}

// Return type tp from e and add signatures (if they exists) and do is true.
func (z *Zone) typeFromElem(elem *tree.Elem, tp uint16, do bool) []dns.RR {
	rrs := elem.Types(tp)
	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, tp)
		if len(sigs) > 0 {
			rrs = append(rrs, sigs...)
		}
	}
	return rrs
}

func (z *Zone) soa(do bool) []dns.RR {
	if do {
		ret := append([]dns.RR{z.Apex.SOA}, z.Apex.SIGSOA...)
		return ret
	}
	return []dns.RR{z.Apex.SOA}
}

func (z *Zone) ns(do bool) []dns.RR {
	if do {
		ret := append(z.Apex.NS, z.Apex.SIGNS...)
		return ret
	}
	return z.Apex.NS
}

func (z *Zone) searchCNAME(elem *tree.Elem, rrs []dns.RR, qtype uint16, do bool) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, dns.TypeCNAME)
		if len(sigs) > 0 {
			rrs = append(rrs, sigs...)
		}
	}

	elem, _ = z.Tree.Search(rrs[0].(*dns.CNAME).Target)
	if elem == nil {
		return rrs, nil, nil, Success
	}

	i := 0

Redo:
	cname := elem.Types(dns.TypeCNAME)
	if len(cname) > 0 {
		rrs = append(rrs, cname...)

		if do {
			sigs := elem.Types(dns.TypeRRSIG)
			sigs = signatureForSubType(sigs, dns.TypeCNAME)
			if len(sigs) > 0 {
				rrs = append(rrs, sigs...)
			}
		}
		elem, _ = z.Tree.Search(cname[0].(*dns.CNAME).Target)
		if elem == nil {
			return rrs, nil, nil, Success
		}

		i++
		if i > maxChain {
			return rrs, nil, nil, Success
		}

		goto Redo
	}

	targets := cnameForType(elem.All(), qtype)
	if len(targets) > 0 {
		rrs = append(rrs, targets...)

		if do {
			sigs := elem.Types(dns.TypeRRSIG)
			sigs = signatureForSubType(sigs, qtype)
			if len(sigs) > 0 {
				rrs = append(rrs, sigs...)
			}
		}
	}

	return rrs, nil, nil, Success
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

// signatureForSubType range through the signature and return the correct ones for the subtype.
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

// Glue returns any potential glue records for nsrrs.
func (z *Zone) Glue(nsrrs []dns.RR) []dns.RR {
	glue := []dns.RR{}
	for _, rr := range nsrrs {
		if ns, ok := rr.(*dns.NS); ok && dns.IsSubDomain(ns.Header().Name, ns.Ns) {
			glue = append(glue, z.searchGlue(ns.Ns)...)
		}
	}
	return glue
}

// searchGlue looks up A and AAAA for name.
func (z *Zone) searchGlue(name string) []dns.RR {
	glue := []dns.RR{}

	// A
	if elem, found := z.Tree.Search(name); found {
		glue = append(glue, elem.Types(dns.TypeA)...)
	}

	// AAAA
	if elem, found := z.Tree.Search(name); found {
		glue = append(glue, elem.Types(dns.TypeAAAA)...)
	}
	return glue
}

const maxChain = 8
