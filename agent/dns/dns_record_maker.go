// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/hashicorp/consul/agent/discovery"
)

// dnsRecordMaker creates DNS records to be used when generating
// responses to dns requests.
type dnsRecordMaker struct{}

// makeSOA returns an SOA record for the given domain and config.
func (dnsRecordMaker) makeSOA(domain string, cfg *RouterDynamicConfig) dns.RR {
	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			// Has to be consistent with MinTTL to avoid invalidation
			Ttl: cfg.SOAConfig.Minttl,
		},
		Ns:      "ns." + domain,
		Serial:  uint32(time.Now().Unix()),
		Mbox:    "hostmaster." + domain,
		Refresh: cfg.SOAConfig.Refresh,
		Retry:   cfg.SOAConfig.Retry,
		Expire:  cfg.SOAConfig.Expire,
		Minttl:  cfg.SOAConfig.Minttl,
	}
}

// makeNS returns an NS record for the given domain and fqdn.
func (dnsRecordMaker) makeNS(domain, fqdn string, ttl uint32) dns.RR {
	return &dns.NS{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns: fqdn,
	}
}

// makeIPBasedRecord returns an A or AAAA record for the given name and IP.
// Note: we might want to pass in the Query Name here, which is used in addr. and virtual. queries
// since there is only ever one result. Right now choosing to leave it off for simplification.
func (dnsRecordMaker) makeIPBasedRecord(name string, addr *dnsAddress, ttl uint32) dns.RR {

	if addr.IsIPV4() {
		// check if the query type is  A for IPv4 or ANY
		return &dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: addr.IP(),
		}
	}

	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		AAAA: addr.IP(),
	}
}

// makeCNAME returns a CNAME record for the given name and target.
func (dnsRecordMaker) makeCNAME(name string, target string, ttl uint32) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target: dns.Fqdn(target),
	}
}

// makeSRV returns an SRV record for the given name and target.
func (dnsRecordMaker) makeSRV(name, target string, weight uint16, ttl uint32, port *discovery.Port) *dns.SRV {
	return &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Priority: 1,
		Weight:   weight,
		Port:     uint16(port.Number),
		Target:   target,
	}
}

// makeTXT returns a TXT record for the given name and result metadata.
func (dnsRecordMaker) makeTXT(name string, metadata map[string]string, ttl uint32) []dns.RR {
	extra := make([]dns.RR, 0, len(metadata))
	for key, value := range metadata {
		txt := value
		if !strings.HasPrefix(strings.ToLower(key), "rfc1035-") {
			txt = encodeKVasRFC1464(key, value)
		}

		extra = append(extra, &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Txt: []string{txt},
		})
	}
	return extra
}

// encodeKVasRFC1464 encodes a key-value pair according to RFC1464
func encodeKVasRFC1464(key, value string) (txt string) {
	// For details on these replacements c.f. https://www.ietf.org/rfc/rfc1464.txt
	key = strings.Replace(key, "`", "``", -1)
	key = strings.Replace(key, "=", "`=", -1)

	// Backquote the leading spaces
	leadingSpacesRE := regexp.MustCompile("^ +")
	numLeadingSpaces := len(leadingSpacesRE.FindString(key))
	key = leadingSpacesRE.ReplaceAllString(key, strings.Repeat("` ", numLeadingSpaces))

	// Backquote the trailing spaces
	numTrailingSpaces := len(trailingSpacesRE.FindString(key))
	key = trailingSpacesRE.ReplaceAllString(key, strings.Repeat("` ", numTrailingSpaces))

	value = strings.Replace(value, "`", "``", -1)

	return key + "=" + value
}
