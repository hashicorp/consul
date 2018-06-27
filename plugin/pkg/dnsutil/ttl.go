package dnsutil

import (
	"time"

	"github.com/coredns/coredns/plugin/pkg/response"

	"github.com/miekg/dns"
)

// MinimalTTL scans the message returns the lowest TTL found taking into the response.Type of the message.
func MinimalTTL(m *dns.Msg, mt response.Type) time.Duration {
	if mt != response.NoError && mt != response.NameError && mt != response.NoData {
		return MinimalDefaultTTL
	}

	// No data to examine, return a short ttl as a fail safe.
	if len(m.Answer)+len(m.Ns)+len(m.Extra) == 0 {
		return MinimalDefaultTTL
	}

	minTTL := MaximumDefaulTTL
	for _, r := range m.Answer {
		switch mt {
		case response.NameError, response.NoData:
			if r.Header().Rrtype == dns.TypeSOA {
				minTTL = time.Duration(r.(*dns.SOA).Minttl) * time.Second
			}
		case response.NoError, response.Delegation:
			if r.Header().Ttl < uint32(minTTL.Seconds()) {
				minTTL = time.Duration(r.Header().Ttl) * time.Second
			}
		}
	}
	for _, r := range m.Ns {
		switch mt {
		case response.NameError, response.NoData:
			if r.Header().Rrtype == dns.TypeSOA {
				minTTL = time.Duration(r.(*dns.SOA).Minttl) * time.Second
			}
		case response.NoError, response.Delegation:
			if r.Header().Ttl < uint32(minTTL.Seconds()) {
				minTTL = time.Duration(r.Header().Ttl) * time.Second
			}
		}
	}

	for _, r := range m.Extra {
		if r.Header().Rrtype == dns.TypeOPT {
			// OPT records use TTL field for extended rcode and flags
			continue
		}
		switch mt {
		case response.NameError, response.NoData:
			if r.Header().Rrtype == dns.TypeSOA {
				minTTL = time.Duration(r.(*dns.SOA).Minttl) * time.Second
			}
		case response.NoError, response.Delegation:
			if r.Header().Ttl < uint32(minTTL.Seconds()) {
				minTTL = time.Duration(r.Header().Ttl) * time.Second
			}
		}
	}
	return minTTL
}

const (
	// MinimalDefaultTTL is the absolute lowest TTL we use in CoreDNS.
	MinimalDefaultTTL = 5 * time.Second
	// MaximumDefaulTTL is the maximum TTL was use on RRsets in CoreDNS.
	MaximumDefaulTTL = 1 * time.Hour
)
