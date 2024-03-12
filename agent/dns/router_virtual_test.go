// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"net"
	"testing"
)

func Test_HandleRequest_Virtual(t *testing.T) {
	testCases := []HandleTestCase{
		{
			name: "test A 'virtual.' query, ipv4 response",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "c000020a.virtual.dc1.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).On("FetchVirtualIP",
					mock.Anything, mock.Anything).Return(&discovery.Result{
					Node: &discovery.Location{Address: "240.0.0.2"},
					Type: discovery.ResultTypeVirtual,
				}, nil)
			},
			validateAndNormalizeExpected: true,
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "c000020a.virtual.dc1.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "c000020a.virtual.dc1.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("240.0.0.2"),
					},
				},
			},
		},
		{
			name: "test A 'virtual.' query, ipv6 response",
			// Since we asked for an A record, the AAAA record that resolves from the address is attached as an extra
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "20010db800010002cafe000000001337.virtual.dc1.consul",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).On("FetchVirtualIP",
					mock.Anything, mock.Anything).Return(&discovery.Result{
					Node: &discovery.Location{Address: "2001:db8:1:2:cafe::1337"},
					Type: discovery.ResultTypeVirtual,
				}, nil)
			},
			validateAndNormalizeExpected: true,
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "20010db800010002cafe000000001337.virtual.dc1.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Extra: []dns.RR{
					&dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   "20010db800010002cafe000000001337.virtual.dc1.consul.",
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						AAAA: net.ParseIP("2001:db8:1:2:cafe::1337"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runHandleTestCases(t, tc)
		})
	}
}
