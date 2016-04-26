package dnssec

import (
	"hash/fnv"
	"strconv"

	"github.com/miekg/dns"
)

// Key serializes the RRset and return a signature cache key.
func key(rrs []dns.RR) string {
	h := fnv.New64()
	buf := make([]byte, 256)
	for _, r := range rrs {
		off, err := dns.PackRR(r, buf, 0, nil, false)
		if err == nil {
			h.Write(buf[:off])
		}
	}

	i := h.Sum64()
	return strconv.FormatUint(i, 10)
}
