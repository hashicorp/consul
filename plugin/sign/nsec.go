package sign

import (
	"sort"

	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/file/tree"

	"github.com/miekg/dns"
)

// names returns the elements of the zone in nsec order.
func names(origin string, z *file.Zone) []string {
	// There will also be apex records other than NS and SOA (who are kept separate), as we
	// are adding DNSKEY and CDS/CDNSKEY records in the apex *before* we sign.
	n := []string{}
	z.AuthWalk(func(e *tree.Elem, _ map[uint16][]dns.RR, auth bool) error {
		if !auth {
			return nil
		}
		n = append(n, e.Name())
		return nil
	})
	return n
}

// NSEC returns an NSEC record according to name, next, ttl and bitmap. Note that the bitmap is sorted before use.
func NSEC(name, next string, ttl uint32, bitmap []uint16) *dns.NSEC {
	sort.Slice(bitmap, func(i, j int) bool { return bitmap[i] < bitmap[j] })

	return &dns.NSEC{
		Hdr:        dns.RR_Header{Name: name, Ttl: ttl, Rrtype: dns.TypeNSEC, Class: dns.ClassINET},
		NextDomain: next,
		TypeBitMap: bitmap,
	}
}
