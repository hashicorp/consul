package autopath

import "github.com/miekg/dns"

// CNAME returns a new CNAME formed from name target and ttl.
func CNAME(name string, target string, ttl uint32) *dns.CNAME {
	return &dns.CNAME{
		Hdr:    dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
		Target: dns.Fqdn(target)}
}
