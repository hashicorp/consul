package cache

import (
	"time"

	"github.com/miekg/coredns/middleware/pkg/response"
	"github.com/miekg/dns"
)

type item struct {
	Rcode              int
	Authoritative      bool
	AuthenticatedData  bool
	RecursionAvailable bool
	Answer             []dns.RR
	Ns                 []dns.RR
	Extra              []dns.RR

	origTTL uint32
	stored  time.Time
}

func newItem(m *dns.Msg, d time.Duration) *item {
	i := new(item)
	i.Rcode = m.Rcode
	i.Authoritative = m.Authoritative
	i.AuthenticatedData = m.AuthenticatedData
	i.RecursionAvailable = m.RecursionAvailable
	i.Answer = m.Answer
	i.Ns = m.Ns
	i.Extra = make([]dns.RR, len(m.Extra))
	// Don't copy OPT record as these are hop-by-hop.
	j := 0
	for _, e := range m.Extra {
		if e.Header().Rrtype == dns.TypeOPT {
			continue
		}
		i.Extra[j] = e
		j++
	}
	i.Extra = i.Extra[:j]

	i.origTTL = uint32(d.Seconds())
	i.stored = time.Now().UTC()

	return i
}

// toMsg turns i into a message, it tailers the reply to m.
// The Autoritative bit is always set to 0, because the answer is from the cache.
func (i *item) toMsg(m *dns.Msg) *dns.Msg {
	m1 := new(dns.Msg)
	m1.SetReply(m)

	m1.Authoritative = false
	m1.AuthenticatedData = i.AuthenticatedData
	m1.RecursionAvailable = i.RecursionAvailable
	m1.Rcode = i.Rcode
	m1.Compress = true

	m1.Answer = i.Answer
	m1.Ns = i.Ns
	m1.Extra = i.Extra

	ttl := int(i.origTTL) - int(time.Now().UTC().Sub(i.stored).Seconds())
	setMsgTTL(m1, uint32(ttl))
	return m1
}

func (i *item) expired(now time.Time) bool {
	ttl := int(i.origTTL) - int(now.UTC().Sub(i.stored).Seconds())
	return ttl < 0
}

// setMsgTTL sets the ttl on all RRs in all sections. If ttl is smaller than minTTL
// that value is used.
func setMsgTTL(m *dns.Msg, ttl uint32) {
	if ttl < minTTL {
		ttl = minTTL
	}

	for _, r := range m.Answer {
		r.Header().Ttl = ttl
	}
	for _, r := range m.Ns {
		r.Header().Ttl = ttl
	}
	for _, r := range m.Extra {
		if r.Header().Rrtype == dns.TypeOPT {
			continue
		}
		r.Header().Ttl = ttl
	}
}

func minMsgTTL(m *dns.Msg, mt response.Type) time.Duration {
	if mt != response.NoError && mt != response.NameError && mt != response.NoData {
		return 0
	}

	minTTL := maxTTL
	for _, r := range append(m.Answer, m.Ns...) {
		switch mt {
		case response.NameError, response.NoData:
			if r.Header().Rrtype == dns.TypeSOA {
				return time.Duration(r.(*dns.SOA).Minttl) * time.Second
			}
		case response.NoError, response.Delegation:
			if r.Header().Ttl < uint32(minTTL.Seconds()) {
				minTTL = time.Duration(r.Header().Ttl) * time.Second
			}
		}
	}
	return minTTL
}
