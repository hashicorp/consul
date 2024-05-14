// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/discovery"
)

func TestDNSRecordMaker_makeSOA(t *testing.T) {
	cfg := &RouterDynamicConfig{
		SOAConfig: SOAConfig{
			Refresh: 1,
			Retry:   2,
			Expire:  3,
			Minttl:  4,
		},
	}
	domain := "testdomain."
	expected := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   "testdomain.",
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    4,
		},
		Ns:      "ns.testdomain.",
		Serial:  uint32(time.Now().Unix()),
		Mbox:    "hostmaster.testdomain.",
		Refresh: 1,
		Retry:   2,
		Expire:  3,
		Minttl:  4,
	}
	actual := dnsRecordMaker{}.makeSOA(domain, cfg)
	require.Equal(t, expected, actual)
}

func TestDNSRecordMaker_makeNS(t *testing.T) {
	domain := "testdomain."
	fqdn := "ns.testdomain."
	ttl := uint32(123)
	expected := &dns.NS{
		Hdr: dns.RR_Header{
			Name:   "testdomain.",
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    123,
		},
		Ns: "ns.testdomain.",
	}
	actual := dnsRecordMaker{}.makeNS(domain, fqdn, ttl)
	require.Equal(t, expected, actual)
}

func TestDNSRecordMaker_makeIPBasedRecord(t *testing.T) {
	ipv4Addr := newDNSAddress("1.2.3.4")
	ipv6Addr := newDNSAddress("2001:db8:1:2:cafe::1337")
	testCases := []struct {
		name             string
		recordHeaderName string
		addr             *dnsAddress
		ttl              uint32
		expected         dns.RR
	}{
		{
			name:             "IPv4",
			recordHeaderName: "my.service.dc1.consul.",
			addr:             ipv4Addr,
			ttl:              123,
			expected: &dns.A{
				Hdr: dns.RR_Header{
					Name:   "my.service.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    123,
				},
				A: ipv4Addr.IP(),
			},
		},
		{
			name:             "IPv6",
			recordHeaderName: "my.service.dc1.consul.",
			addr:             ipv6Addr,
			ttl:              123,
			expected: &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   "my.service.dc1.consul.",
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    123,
				},
				AAAA: ipv6Addr.IP(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := dnsRecordMaker{}.makeIPBasedRecord(tc.recordHeaderName, tc.addr, tc.ttl)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestDNSRecordMaker_makeCNAME(t *testing.T) {
	name := "my.service.consul."
	target := "foo"
	ttl := uint32(123)
	expected := &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   "my.service.consul.",
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    123,
		},
		Target: "foo.",
	}
	actual := dnsRecordMaker{}.makeCNAME(name, target, ttl)
	require.Equal(t, expected, actual)
}

func TestDNSRecordMaker_makeSRV(t *testing.T) {
	name := "my.service.consul."
	target := "foo"
	ttl := uint32(123)
	expected := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   "my.service.consul.",
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    123,
		},
		Priority: 1,
		Weight:   uint16(345),
		Port:     uint16(234),
		Target:   "foo",
	}
	actual := dnsRecordMaker{}.makeSRV(name, target, uint16(345), ttl, &discovery.Port{Number: 234})
	require.Equal(t, expected, actual)
}

func TestDNSRecordMaker_makeTXT(t *testing.T) {
	testCases := []struct {
		name     string
		metadata map[string]string
		ttl      uint32
		expected []dns.RR
	}{
		{
			name: "single metadata",
			metadata: map[string]string{
				"key": "value",
			},
			ttl: 123,
			expected: []dns.RR{
				&dns.TXT{
					Hdr: dns.RR_Header{
						Name:   "my.service.consul.",
						Rrtype: dns.TypeTXT,
						Class:  dns.ClassINET,
						Ttl:    123,
					},
					Txt: []string{"key=value"},
				},
			},
		},
		{
			name: "multiple metadata entries",
			metadata: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			ttl: 123,
			expected: []dns.RR{
				&dns.TXT{
					Hdr: dns.RR_Header{
						Name:   "my.service.consul.",
						Rrtype: dns.TypeTXT,
						Class:  dns.ClassINET,
						Ttl:    123,
					},
					Txt: []string{"key1=value1"},
				},
				&dns.TXT{
					Hdr: dns.RR_Header{
						Name:   "my.service.consul.",
						Rrtype: dns.TypeTXT,
						Class:  dns.ClassINET,
						Ttl:    123,
					},
					Txt: []string{"key2=value2"},
				},
			},
		},
		{
			name: "'rfc1035-' prefixed- metadata entry",
			metadata: map[string]string{
				"rfc1035-key": "value",
			},
			ttl: 123,
			expected: []dns.RR{
				&dns.TXT{
					Hdr: dns.RR_Header{
						Name:   "my.service.consul.",
						Rrtype: dns.TypeTXT,
						Class:  dns.ClassINET,
						Ttl:    123,
					},
					Txt: []string{"value"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := dnsRecordMaker{}.makeTXT("my.service.consul.", tc.metadata, tc.ttl)
			require.ElementsMatchf(t, tc.expected, actual, "expected: %v, actual: %v", tc.expected, actual)
		})
	}
}
