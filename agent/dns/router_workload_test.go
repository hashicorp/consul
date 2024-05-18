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

func Test_HandleRequest_workloads(t *testing.T) {
	testCases := []HandleTestCase{
		{
			name: "workload A query w/ port, returns A record",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := &discovery.Result{
					Node:    &discovery.Location{Name: "foo", Address: "1.2.3.4"},
					Type:    discovery.ResultTypeWorkload,
					Tenancy: discovery.ResultTenancy{},
					Ports: []discovery.Port{
						{
							Name:   "api",
							Number: 5678,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchWorkload", mock.Anything, mock.Anything).
					Return(result, nil). //TODO
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)

						require.Equal(t, "foo", req.Name)
						require.Equal(t, "api", req.PortName)
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
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "api.port.foo.workload.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "workload ANY query w/o port, returns A record",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.workload.consul.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := &discovery.Result{
					Node:    &discovery.Location{Name: "foo", Address: "1.2.3.4"},
					Type:    discovery.ResultTypeWorkload,
					Tenancy: discovery.ResultTenancy{},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchWorkload", mock.Anything, mock.Anything).
					Return(result, nil). //TODO
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)

						require.Equal(t, "foo", req.Name)
						require.Empty(t, req.PortName)
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
						Name:   "foo.workload.consul.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.workload.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "workload A query with namespace, partition, and cluster id; IPV4 address; returns A record",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.workload.bar.ns.baz.ap.dc3.dc.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := &discovery.Result{
					Node: &discovery.Location{Name: "foo", Address: "1.2.3.4"},
					Type: discovery.ResultTypeWorkload,
					Tenancy: discovery.ResultTenancy{
						Namespace: "bar",
						Partition: "baz",
						// We currently don't set the datacenter in any of the V2 results.
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchWorkload", mock.Anything, mock.Anything).
					Return(result, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)

						require.Equal(t, "foo", req.Name)
						require.Empty(t, req.PortName)
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
						Name:   "foo.workload.bar.ns.baz.ap.dc3.dc.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.workload.bar.ns.baz.ap.dc3.dc.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "workload w/hostname address, ANY query (no recursor)",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := &discovery.Result{
					Node:    &discovery.Location{Name: "foo", Address: "foo.example.com"},
					Type:    discovery.ResultTypeWorkload,
					Tenancy: discovery.ResultTenancy{},
					Ports: []discovery.Port{
						{
							Name:   "api",
							Number: 5678,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchWorkload", mock.Anything, mock.Anything).
					Return(result, nil). //TODO
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)

						require.Equal(t, "foo", req.Name)
						require.Equal(t, "api", req.PortName)
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
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.CNAME{
						Hdr: dns.RR_Header{
							Name:   "api.port.foo.workload.consul.",
							Rrtype: dns.TypeCNAME,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Target: "foo.example.com.",
					},
				},
			},
		},
		{
			name: "workload w/hostname address, ANY query (w/ recursor)",
			// https://datatracker.ietf.org/doc/html/rfc1034#section-3.6.2 both the CNAME and the A record should be in the answer
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := &discovery.Result{
					Node:    &discovery.Location{Name: "foo", Address: "foo.example.com"},
					Type:    discovery.ResultTypeWorkload,
					Tenancy: discovery.ResultTenancy{},
					Ports: []discovery.Port{
						{
							Name:   "api",
							Number: 5678,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchWorkload", mock.Anything, mock.Anything).
					Return(result, nil). //TODO
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)

						require.Equal(t, "foo", req.Name)
						require.Equal(t, "api", req.PortName)
					})
			},
			agentConfig: &config.RuntimeConfig{
				DNSRecursors: []string{"8.8.8.8"},
				DNSDomain:    "consul",
				DNSNodeTTL:   123 * time.Second,
				DNSSOA: config.RuntimeSOAConfig{
					Refresh: 1,
					Retry:   2,
					Expire:  3,
					Minttl:  4,
				},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
			},
			configureRecursor: func(recursor dnsRecursor) {
				resp := &dns.Msg{
					MsgHdr: dns.MsgHdr{
						Opcode:        dns.OpcodeQuery,
						Response:      true,
						Authoritative: true,
						Rcode:         dns.RcodeSuccess,
					},
					Question: []dns.Question{
						{
							Name:   "foo.example.com.",
							Qtype:  dns.TypeA,
							Qclass: dns.ClassINET,
						},
					},
					Answer: []dns.RR{
						&dns.A{
							Hdr: dns.RR_Header{
								Name:   "foo.example.com.",
								Rrtype: dns.TypeA,
								Class:  dns.ClassINET,
							},
							A: net.ParseIP("1.2.3.4"),
						},
					},
				}
				recursor.(*mockDnsRecursor).On("handle",
					mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)
			},
			validateAndNormalizeExpected: true,
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:             dns.OpcodeQuery,
					Response:           true,
					Authoritative:      true,
					RecursionAvailable: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.CNAME{
						Hdr: dns.RR_Header{
							Name:   "api.port.foo.workload.consul.",
							Rrtype: dns.TypeCNAME,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Target: "foo.example.com.",
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.example.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "workload w/hostname address, CNAME query (w/ recursor)",
			// https://datatracker.ietf.org/doc/html/rfc1034#section-3.6.2 only the CNAME should be in the answer
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeCNAME,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := &discovery.Result{
					Node:    &discovery.Location{Name: "foo", Address: "foo.example.com"},
					Type:    discovery.ResultTypeWorkload,
					Tenancy: discovery.ResultTenancy{},
					Ports: []discovery.Port{
						{
							Name:   "api",
							Number: 5678,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchWorkload", mock.Anything, mock.Anything).
					Return(result, nil). //TODO
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)

						require.Equal(t, "foo", req.Name)
						require.Equal(t, "api", req.PortName)
					})
			},
			agentConfig: &config.RuntimeConfig{
				DNSRecursors: []string{"8.8.8.8"},
				DNSDomain:    "consul",
				DNSNodeTTL:   123 * time.Second,
				DNSSOA: config.RuntimeSOAConfig{
					Refresh: 1,
					Retry:   2,
					Expire:  3,
					Minttl:  4,
				},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
			},
			configureRecursor: func(recursor dnsRecursor) {
				resp := &dns.Msg{
					MsgHdr: dns.MsgHdr{
						Opcode:        dns.OpcodeQuery,
						Response:      true,
						Authoritative: true,
						Rcode:         dns.RcodeSuccess,
					},
					Question: []dns.Question{
						{
							Name:   "foo.example.com.",
							Qtype:  dns.TypeA,
							Qclass: dns.ClassINET,
						},
					},
					Answer: []dns.RR{
						&dns.A{
							Hdr: dns.RR_Header{
								Name:   "foo.example.com.",
								Rrtype: dns.TypeCNAME,
								Class:  dns.ClassINET,
							},
							A: net.ParseIP("1.2.3.4"),
						},
					},
				}
				recursor.(*mockDnsRecursor).On("handle",
					mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)
			},
			validateAndNormalizeExpected: true,
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:             dns.OpcodeQuery,
					Response:           true,
					Authoritative:      true,
					RecursionAvailable: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "api.port.foo.workload.consul.",
						Qtype:  dns.TypeCNAME,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.CNAME{
						Hdr: dns.RR_Header{
							Name:   "api.port.foo.workload.consul.",
							Rrtype: dns.TypeCNAME,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Target: "foo.example.com.",
					},
					// TODO (v2-dns): this next record is wrong per the RFC-1034 mentioned in the comment above (NET-8060)
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.example.com.",
							Rrtype: dns.TypeCNAME,
							Class:  dns.ClassINET,
							Ttl:    123,
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
