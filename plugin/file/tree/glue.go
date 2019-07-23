package tree

import (
	"github.com/coredns/coredns/plugin/file/rrutil"

	"github.com/miekg/dns"
)

// Glue returns any potential glue records for nsrrs.
func (t *Tree) Glue(nsrrs []dns.RR, do bool) []dns.RR {
	glue := []dns.RR{}
	for _, rr := range nsrrs {
		if ns, ok := rr.(*dns.NS); ok && dns.IsSubDomain(ns.Header().Name, ns.Ns) {
			glue = append(glue, t.searchGlue(ns.Ns, do)...)
		}
	}
	return glue
}

// searchGlue looks up A and AAAA for name.
func (t *Tree) searchGlue(name string, do bool) []dns.RR {
	glue := []dns.RR{}

	// A
	if elem, found := t.Search(name); found {
		glue = append(glue, elem.Type(dns.TypeA)...)
		if do {
			sigs := elem.Type(dns.TypeRRSIG)
			sigs = rrutil.SubTypeSignature(sigs, dns.TypeA)
			glue = append(glue, sigs...)
		}
	}

	// AAAA
	if elem, found := t.Search(name); found {
		glue = append(glue, elem.Type(dns.TypeAAAA)...)
		if do {
			sigs := elem.Type(dns.TypeRRSIG)
			sigs = rrutil.SubTypeSignature(sigs, dns.TypeAAAA)
			glue = append(glue, sigs...)
		}
	}
	return glue
}
