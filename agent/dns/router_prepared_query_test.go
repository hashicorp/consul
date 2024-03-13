// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/discovery"
)

func Test_HandleRequest_PreparedQuery(t *testing.T) {
	testCases := []HandleTestCase{
		{
			name: "v1 prepared query w/ TTL override, ANY query, returns A record",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.query.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			agentConfig: &config.RuntimeConfig{
				DNSDomain:  "consul",
				DNSNodeTTL: 123 * time.Second,
				DNSSOA: config.RuntimeSOAConfig{
					Refresh: 1,
					Retry:   2,
					Expire:  3,
					Minttl:  4,
				},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
				// We shouldn't use this if we have the override defined
				DNSServiceTTL: map[string]time.Duration{
					"foo": 1 * time.Second,
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchPreparedQuery", mock.Anything, mock.Anything).
					Return([]*discovery.Result{
						{
							Service: &discovery.Location{Name: "foo", Address: "1.2.3.4"},
							Node:    &discovery.Location{Name: "bar", Address: "1.2.3.4"},
							Type:    discovery.ResultTypeService,
							Tenancy: discovery.ResultTenancy{
								Datacenter: "dc1",
							},
							DNS: discovery.DNSConfig{
								TTL:    getUint32Ptr(3),
								Weight: 1,
							},
						},
					}, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						require.Equal(t, "foo", req.Name)
					})
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
						Name:   "foo.query.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.query.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    3,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "v1 prepared query w/ matching service TTL, ANY query, returns A record",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.query.dc1.cluster.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			agentConfig: &config.RuntimeConfig{
				DNSDomain:  "consul",
				DNSNodeTTL: 123 * time.Second,
				DNSSOA: config.RuntimeSOAConfig{
					Refresh: 1,
					Retry:   2,
					Expire:  3,
					Minttl:  4,
				},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
				// Results should use this as the TTL
				DNSServiceTTL: map[string]time.Duration{
					"foo": 1 * time.Second,
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchPreparedQuery", mock.Anything, mock.Anything).
					Return([]*discovery.Result{
						{
							Service: &discovery.Location{Name: "foo", Address: "1.2.3.4"},
							Node:    &discovery.Location{Name: "bar", Address: "1.2.3.4"},
							Type:    discovery.ResultTypeService,
							Tenancy: discovery.ResultTenancy{
								Datacenter: "dc1",
							},
							DNS: discovery.DNSConfig{
								// Intentionally no TTL here.
								Weight: 1,
							},
						},
					}, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						require.Equal(t, "foo", req.Name)
						require.Equal(t, "dc1", req.Tenancy.Datacenter)
					})
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
						Name:   "foo.query.dc1.cluster.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.query.dc1.cluster.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    1,
						},
						A: net.ParseIP("1.2.3.4"),
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
