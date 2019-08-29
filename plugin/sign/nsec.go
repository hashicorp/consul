package sign

import (
	"sort"

	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/file/tree"

	"github.com/miekg/dns"
)

// names returns the elements of the zone in nsec order. If the returned boolean is true there were
// no other apex records than SOA and NS, which are stored separately.
func names(origin string, z *file.Zone) ([]string, bool) {
	// if there are no apex records other than NS and SOA we'll miss the origin
	// in this list. Check the first element and if not origin prepend it.
	n := []string{}
	z.Walk(func(e *tree.Elem, _ map[uint16][]dns.RR) error {
		n = append(n, e.Name())
		return nil
	})
	if len(n) == 0 {
		return nil, false
	}
	if n[0] != origin {
		n = append([]string{origin}, n...)
		return n, true
	}
	return n, false
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
