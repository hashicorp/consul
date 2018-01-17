package cache

import (
	"time"

	"github.com/coredns/coredns/plugin/cache/freq"
	"github.com/coredns/coredns/plugin/pkg/response"
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

	*freq.Freq
}

func newItem(m *dns.Msg, now time.Time, d time.Duration) *item {
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
	i.stored = now.UTC()

	i.Freq = new(freq.Freq)

	return i
}

// toMsg turns i into a message, it tailors the reply to m.
// The Authoritative bit is always set to 0, because the answer is from the cache.
func (i *item) toMsg(m *dns.Msg, now time.Time) *dns.Msg {
	m1 := new(dns.Msg)
	m1.SetReply(m)

	m1.Authoritative = false
	m1.AuthenticatedData = i.AuthenticatedData
	m1.RecursionAvailable = i.RecursionAvailable
	m1.Rcode = i.Rcode
	m1.Compress = true

	m1.Answer = make([]dns.RR, len(i.Answer))
	m1.Ns = make([]dns.RR, len(i.Ns))
	m1.Extra = make([]dns.RR, len(i.Extra))

	ttl := uint32(i.ttl(now))
	for j, r := range i.Answer {
		m1.Answer[j] = dns.Copy(r)
		m1.Answer[j].Header().Ttl = ttl
	}
	for j, r := range i.Ns {
		m1.Ns[j] = dns.Copy(r)
		m1.Ns[j].Header().Ttl = ttl
	}
	for j, r := range i.Extra {
		m1.Extra[j] = dns.Copy(r)
		if m1.Extra[j].Header().Rrtype != dns.TypeOPT {
			m1.Extra[j].Header().Ttl = ttl
		}
	}
	return m1
}

func (i *item) ttl(now time.Time) int {
	ttl := int(i.origTTL) - int(now.UTC().Sub(i.stored).Seconds())
	return ttl
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
