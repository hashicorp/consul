package dnssec

import (
	"hash/fnv"

	"github.com/miekg/dns"
)

// hash serializes the RRset and return a signature cache key.
func hash(rrs []dns.RR) uint64 {
	h := fnv.New64()
	buf := make([]byte, 256)
	for _, r := range rrs {
		off, err := dns.PackRR(r, buf, 0, nil, false)
		if err == nil {
			h.Write(buf[:off])
		}
	}

	i := h.Sum64()
	return i
}
