package dnssec

import "github.com/miekg/dns"

// nsec returns an NSEC useful for NXDOMAIN respsones.
// See https://tools.ietf.org/html/draft-valsorda-dnsop-black-lies-00
// For example, a request for the non-existing name a.example.com would
// cause the following NSEC record to be generated:
//	a.example.com. 3600 IN NSEC \000.a.example.com. ( RRSIG NSEC )
// This inturn makes every NXDOMAIN answer a NODATA one, don't forget to flip
// the header rcode to NOERROR.
func (d Dnssec) nsec(name, zone string, ttl, incep, expir uint32) ([]dns.RR, error) {
	nsec := &dns.NSEC{}
	nsec.Hdr = dns.RR_Header{Name: name, Ttl: ttl, Class: dns.ClassINET, Rrtype: dns.TypeNSEC}
	nsec.NextDomain = "\\000." + name
	nsec.TypeBitMap = []uint16{dns.TypeRRSIG, dns.TypeNSEC}

	sigs, err := d.sign([]dns.RR{nsec}, zone, ttl, incep, expir)
	if err != nil {
		return nil, err
	}

	return append(sigs, nsec), nil
}
