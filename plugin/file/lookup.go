package file

import (
	"github.com/coredns/coredns/plugin/file/tree"
	"github.com/coredns/coredns/request"

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
func (z *Zone) Lookup(state request.Request, qname string) ([]dns.RR, []dns.RR, []dns.RR, Result) {

	qtype := state.QType()
	do := state.Do()

	if !z.NoReload {
		z.reloadMu.RLock()
	}
	defer func() {
		if !z.NoReload {
			z.reloadMu.RUnlock()
		}
	}()

	// If z is a secondary zone we might not have transferred it, meaning we have
	// all zone context setup, except the actual record. This means (for one thing) the apex
	// is empty and we don't have a SOA record.
	soa := z.Apex.SOA
	if soa == nil {
		return nil, nil, nil, ServerFailure
	}

	if qtype == dns.TypeSOA {
		return z.soa(do), z.ns(do), nil, Success
	}
	if qtype == dns.TypeNS && qname == z.origin {
		nsrrs := z.ns(do)
		glue := z.Glue(nsrrs, do)
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
	// If found we check if it is a CNAME/DNAME and do CNAME processing
	// We also check if we have type and do a nodata resposne.
	//
	// If not found, we check the potential wildcard, and use that for further processing.
	// If not found and no wildcard we will process this as an NXDOMAIN response.
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
			// wildcard we should expand the wildcard.

			wildcard := replaceWithAsteriskLabel(parts)
			if wild, found := z.Tree.Search(wildcard); found {
				wildElem = wild
			}

			// Keep on searching, because maybe we hit an empty-non-terminal (which aren't
			// stored in the tree. Only when we have match the full qname (and possible wildcard
			// we can be confident that we didn't find anything.
			i++
			continue
		}

		// If we see DNAME records, we should return those.
		if dnamerrs := elem.Types(dns.TypeDNAME); dnamerrs != nil {
			// Only one DNAME is allowed per name. We just pick the first one to synthesize from.
			dname := dnamerrs[0]
			if cname := synthesizeCNAME(state.Name(), dname.(*dns.DNAME)); cname != nil {
				answer, ns, extra, rcode := z.searchCNAME(state, elem, []dns.RR{cname})

				if do {
					sigs := elem.Types(dns.TypeRRSIG)
					sigs = signatureForSubType(sigs, dns.TypeDNAME)
					dnamerrs = append(dnamerrs, sigs...)
				}

				// The relevant DNAME RR should be included in the answer section,
				// if the DNAME is being employed as a substitution instruction.
				answer = append(dnamerrs, answer...)

				return answer, ns, extra, rcode
			}
			// The domain name that owns a DNAME record is allowed to have other RR types
			// at that domain name, except those have restrictions on what they can coexist
			// with (e.g. another DNAME). So there is nothing special left here.
		}

		// If we see NS records, it means the name as been delegated, and we should return the delegation.
		if nsrrs := elem.Types(dns.TypeNS); nsrrs != nil {

			// If the query is specifically for DS and the qname matches the delegated name, we should
			// return the DS in the answer section and leave the rest empty, i.e. just continue the loop
			// and continue searching.
			if qtype == dns.TypeDS && elem.Name() == qname {
				i++
				continue
			}

			glue := z.Glue(nsrrs, do)
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

		if rrs := elem.Types(dns.TypeCNAME); len(rrs) > 0 && qtype != dns.TypeCNAME {
			return z.searchCNAME(state, elem, rrs)
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

		// Additional section processing for MX, SRV. Check response and see if any of the names are in baliwick -
		// if so add IP addresses to the additional section.
		additional := additionalProcessing(z, rrs, do)

		if do {
			sigs := elem.Types(dns.TypeRRSIG)
			sigs = signatureForSubType(sigs, qtype)
			rrs = append(rrs, sigs...)
		}

		return rrs, z.ns(do), additional, Success

	}

	// Haven't found the original name.

	// Found wildcard.
	if wildElem != nil {
		auth := z.ns(do)

		if rrs := wildElem.Types(dns.TypeCNAME, qname); len(rrs) > 0 {
			return z.searchCNAME(state, wildElem, rrs)
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
	// must be an empty-non-terminal. If so, we do the proper NXDOMAIN handling, but set the rcode to be success.
	if x, found := z.Tree.Next(qname); found {
		if dns.IsSubDomain(qname, x.Name()) {
			rcode = Success
		}
	}

	ret := z.soa(do)
	if do {
		deny, found := z.Tree.Prev(qname)
		if !found {
			goto Out
		}
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

// TODO(miek): should be better named, like aditionalProcessing?
func (z *Zone) searchCNAME(state request.Request, elem *tree.Elem, rrs []dns.RR) ([]dns.RR, []dns.RR, []dns.RR, Result) {

	qtype := state.QType()
	do := state.Do()

	if do {
		sigs := elem.Types(dns.TypeRRSIG)
		sigs = signatureForSubType(sigs, dns.TypeCNAME)
		if len(sigs) > 0 {
			rrs = append(rrs, sigs...)
		}
	}

	targetName := rrs[0].(*dns.CNAME).Target
	elem, _ = z.Tree.Search(targetName)
	if elem == nil {
		if !dns.IsSubDomain(z.origin, targetName) {
			rrs = append(rrs, z.externalLookup(state, targetName, qtype)...)
		}
		return rrs, z.ns(do), nil, Success
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
		targetName := cname[0].(*dns.CNAME).Target
		elem, _ = z.Tree.Search(targetName)
		if elem == nil {
			if !dns.IsSubDomain(z.origin, targetName) {
				if !dns.IsSubDomain(z.origin, targetName) {
					rrs = append(rrs, z.externalLookup(state, targetName, qtype)...)
				}
			}
			return rrs, z.ns(do), nil, Success
		}

		i++
		if i > maxChain {
			return rrs, z.ns(do), nil, Success
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

	return rrs, z.ns(do), nil, Success
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

func (z *Zone) externalLookup(state request.Request, target string, qtype uint16) []dns.RR {
	m, e := z.Proxy.Lookup(state, target, qtype)
	if e != nil {
		// TODO(miek): debugMsg for this as well? Log?
		return nil
	}
	return m.Answer
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
func (z *Zone) Glue(nsrrs []dns.RR, do bool) []dns.RR {
	glue := []dns.RR{}
	for _, rr := range nsrrs {
		if ns, ok := rr.(*dns.NS); ok && dns.IsSubDomain(ns.Header().Name, ns.Ns) {
			glue = append(glue, z.searchGlue(ns.Ns, do)...)
		}
	}
	return glue
}

// searchGlue looks up A and AAAA for name.
func (z *Zone) searchGlue(name string, do bool) []dns.RR {
	glue := []dns.RR{}

	// A
	if elem, found := z.Tree.Search(name); found {
		glue = append(glue, elem.Types(dns.TypeA)...)
		if do {
			sigs := elem.Types(dns.TypeRRSIG)
			sigs = signatureForSubType(sigs, dns.TypeA)
			glue = append(glue, sigs...)
		}
	}

	// AAAA
	if elem, found := z.Tree.Search(name); found {
		glue = append(glue, elem.Types(dns.TypeAAAA)...)
		if do {
			sigs := elem.Types(dns.TypeRRSIG)
			sigs = signatureForSubType(sigs, dns.TypeAAAA)
			glue = append(glue, sigs...)
		}
	}
	return glue
}

// additionalProcessing checks the current answer section and retrieves A or AAAA records
// (and possible SIGs) to need to be put in the additional section.
func additionalProcessing(z *Zone, answer []dns.RR, do bool) (extra []dns.RR) {
	for _, rr := range answer {
		name := ""
		switch x := rr.(type) {
		case *dns.SRV:
			name = x.Target
		case *dns.MX:
			name = x.Mx
		}
		if !dns.IsSubDomain(z.origin, name) {
			continue
		}

		elem, _ := z.Tree.Search(name)
		if elem == nil {
			continue
		}

		sigs := elem.Types(dns.TypeRRSIG)
		for _, addr := range []uint16{dns.TypeA, dns.TypeAAAA} {
			if a := elem.Types(addr); a != nil {
				extra = append(extra, a...)
				if do {
					sig := signatureForSubType(sigs, addr)
					extra = append(extra, sig...)
				}
			}
		}
	}

	return extra
}

const maxChain = 8
