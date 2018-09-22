package file

import (
	"github.com/coredns/coredns/plugin/pkg/dnsutil"

	"github.com/miekg/dns"
)

// substituteDNAME performs the DNAME substitution defined by RFC 6672,
// assuming the QTYPE of the query is not DNAME. It returns an empty
// string if there is no match.
func substituteDNAME(qname, owner, target string) string {
	if dns.IsSubDomain(owner, qname) && qname != owner {
		labels := dns.SplitDomainName(qname)
		labels = append(labels[0:len(labels)-dns.CountLabel(owner)], dns.SplitDomainName(target)...)

		return dnsutil.Join(labels...)
	}

	return ""
}

// synthesizeCNAME returns a CNAME RR pointing to the resulting name of
// the DNAME substitution. The owner name of the CNAME is the QNAME of
// the query and the TTL is the same as the corresponding DNAME RR.
//
// It returns nil if the DNAME substitution has no match.
func synthesizeCNAME(qname string, d *dns.DNAME) *dns.CNAME {
	target := substituteDNAME(qname, d.Header().Name, d.Target)
	if target == "" {
		return nil
	}

	r := new(dns.CNAME)
	r.Hdr = dns.RR_Header{
		Name:   qname,
		Rrtype: dns.TypeCNAME,
		Class:  dns.ClassINET,
		Ttl:    d.Header().Ttl,
	}
	r.Target = target

	return r
}
