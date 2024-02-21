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
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
)

func Test_HandleRequest_NS(t *testing.T) {
	testCases := []HandleTestCase{
		{
			name: "vanilla NS query",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "consul.",
						Qtype:  dns.TypeNS,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return([]*discovery.Result{
						{
							Node: &discovery.Location{Name: "server-one", Address: "1.2.3.4"},
							Type: discovery.ResultTypeWorkload,
							Tenancy: discovery.ResultTenancy{
								Namespace: resource.DefaultNamespaceName,
								Partition: resource.DefaultPartitionName,
							},
						},
						{
							Node: &discovery.Location{Name: "server-two", Address: "4.5.6.7"},
							Type: discovery.ResultTypeWorkload,
							Tenancy: discovery.ResultTenancy{
								Namespace: resource.DefaultNamespaceName,
								Partition: resource.DefaultPartitionName,
							},
						},
					}, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, discovery.LookupTypeService, reqType)
						require.Equal(t, structs.ConsulServiceName, req.Name)
						require.Equal(t, 3, req.Limit)
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
						Name:   "consul.",
						Qtype:  dns.TypeNS,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "consul.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-one.workload.default.ns.default.ap.consul.",
					},
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "consul.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-two.workload.default.ns.default.ap.consul.",
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-one.workload.default.ns.default.ap.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-two.workload.default.ns.default.ap.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("4.5.6.7"),
					},
				},
			},
		},
		{
			name: "NS query against alternate domain",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "testdomain.",
						Qtype:  dns.TypeNS,
						Qclass: dns.ClassINET,
					},
				},
			},
			agentConfig: &config.RuntimeConfig{
				DNSDomain:    "consul",
				DNSAltDomain: "testdomain",
				DNSNodeTTL:   123 * time.Second,
				DNSSOA: config.RuntimeSOAConfig{
					Refresh: 1,
					Retry:   2,
					Expire:  3,
					Minttl:  4,
				},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return([]*discovery.Result{
						{
							Node: &discovery.Location{Name: "server-one", Address: "1.2.3.4"},
							Type: discovery.ResultTypeWorkload,
							Tenancy: discovery.ResultTenancy{
								Namespace: resource.DefaultNamespaceName,
								Partition: resource.DefaultPartitionName,
							},
						},
						{
							Node: &discovery.Location{Name: "server-two", Address: "4.5.6.7"},
							Type: discovery.ResultTypeWorkload,
							Tenancy: discovery.ResultTenancy{
								Namespace: resource.DefaultNamespaceName,
								Partition: resource.DefaultPartitionName,
							},
						},
					}, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, discovery.LookupTypeService, reqType)
						require.Equal(t, structs.ConsulServiceName, req.Name)
						require.Equal(t, 3, req.Limit)
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
						Name:   "testdomain.",
						Qtype:  dns.TypeNS,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-one.workload.default.ns.default.ap.testdomain.",
					},
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-two.workload.default.ns.default.ap.testdomain.",
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-one.workload.default.ns.default.ap.testdomain.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-two.workload.default.ns.default.ap.testdomain.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("4.5.6.7"),
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
