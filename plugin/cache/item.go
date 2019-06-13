package cache

import (
	"time"

	"github.com/coredns/coredns/plugin/cache/freq"
	"github.com/miekg/dns"
)

type item struct {
	Rcode              int
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
	i.AuthenticatedData = m.AuthenticatedData
	i.RecursionAvailable = m.RecursionAvailable
	i.Answer = m.Answer
	i.Ns = m.Ns
	i.Extra = make([]dns.RR, len(m.Extra))
	// Don't copy OPT records as these are hop-by-hop.
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

	// Set this to true as some DNS clients disgard the *entire* packet when it's non-authoritative.
	// This is probably not according to spec, but the bit itself is not super useful as this point, so
	// just set it to true.
	m1.Authoritative = true
	m1.AuthenticatedData = i.AuthenticatedData
	m1.RecursionAvailable = i.RecursionAvailable
	m1.Rcode = i.Rcode

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
	// newItem skips OPT records, so we can just use i.Extra as is.
	for j, r := range i.Extra {
		m1.Extra[j] = dns.Copy(r)
		m1.Extra[j].Header().Ttl = ttl
	}
	return m1
}

func (i *item) ttl(now time.Time) int {
	ttl := int(i.origTTL) - int(now.UTC().Sub(i.stored).Seconds())
	return ttl
}
