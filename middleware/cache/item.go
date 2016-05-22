package cache

import (
	"strconv"
	"time"

	"github.com/miekg/dns"
)

type item struct {
	Authoritative      bool
	AuthenticatedData  bool
	RecursionAvailable bool
	Answer             []dns.RR
	Ns                 []dns.RR
	Extra              []dns.RR

	origTtl uint32
	stored  time.Time
}

func newItem(m *dns.Msg, d time.Duration) *item {
	i := new(item)
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

	i.origTtl = uint32(d.Seconds())
	i.stored = time.Now().UTC()

	return i
}

// toMsg turns i into a message, it tailers to reply to m.
func (i *item) toMsg(m *dns.Msg) *dns.Msg {
	m1 := new(dns.Msg)
	m1.SetReply(m)
	m1.Authoritative = i.Authoritative
	m1.AuthenticatedData = i.AuthenticatedData
	m1.RecursionAvailable = i.RecursionAvailable
	m1.Compress = true

	m1.Answer = i.Answer
	m1.Ns = i.Ns
	m1.Extra = i.Extra

	ttl := int(i.origTtl) - int(time.Now().UTC().Sub(i.stored).Seconds())
	if ttl < baseTtl {
		ttl = baseTtl
	}
	setCap(m1, uint32(ttl))
	return m1
}

// setCap sets the ttl on all RRs in all sections.
func setCap(m *dns.Msg, ttl uint32) {
	for _, r := range m.Answer {
		r.Header().Ttl = uint32(ttl)
	}
	for _, r := range m.Ns {
		r.Header().Ttl = uint32(ttl)
	}
	for _, r := range m.Extra {
		if r.Header().Rrtype == dns.TypeOPT {
			continue
		}
		r.Header().Ttl = uint32(ttl)
	}
}

// nodataKey returns a caching key for NODATA responses.
func noDataKey(qname string, qtype uint16, do bool) string {
	if do {
		return "1" + qname + ".." + strconv.Itoa(int(qtype))
	}
	return "0" + qname + ".." + strconv.Itoa(int(qtype))
}

// nameErrorKey returns a caching key for NXDOMAIN responses.
func nameErrorKey(qname string, do bool) string {
	if do {
		return "1" + qname
	}
	return "0" + qname
}

// successKey returns a caching key for successfull answers.
func successKey(qname string, qtype uint16, do bool) string { return noDataKey(qname, qtype, do) }
