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

	"github.com/hashicorp/consul/agent/discovery"
)

func Test_HandleRequest_PTR(t *testing.T) {
	testCases := []HandleTestCase{
		{
			name: "PTR lookup for node, query type is ANY",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node:    &discovery.Location{Name: "foo", Address: "1.2.3.4"},
						Service: &discovery.Location{Name: "bar", Address: "foo"},
						Type:    discovery.ResultTypeNode,
						Tenancy: discovery.ResultTenancy{
							Datacenter: "dc2",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchRecordsByIp", mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(net.IP)

						require.NotNil(t, req)
						require.Equal(t, "1.2.3.4", req.String())
					})
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.PTR{
						Hdr: dns.RR_Header{
							Name:   "4.3.2.1.in-addr.arpa.",
							Rrtype: dns.TypePTR,
							Class:  dns.ClassINET,
						},
						Ptr: "foo.node.dc2.consul.",
					},
				},
			},
		},
		{
			name: "PTR lookup for IPV6 node",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node:    &discovery.Location{Name: "foo", Address: "2001:db8::567:89ab"},
						Service: &discovery.Location{Name: "web", Address: "foo"},
						Type:    discovery.ResultTypeNode,
						Tenancy: discovery.ResultTenancy{
							Datacenter: "dc2",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchRecordsByIp", mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(net.IP)

						require.NotNil(t, req)
						require.Equal(t, "2001:db8::567:89ab", req.String())
					})
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.PTR{
						Hdr: dns.RR_Header{
							Name:   "b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
							Rrtype: dns.TypePTR,
							Class:  dns.ClassINET,
						},
						Ptr: "foo.node.dc2.consul.",
					},
				},
			},
		},
		{
			name: "PTR lookup for invalid IP address",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "257.3.2.1.in-addr.arpa",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNameError,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "257.3.2.1.in-addr.arpa.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "consul.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    4,
						},
						Ns:      "ns.consul.",
						Serial:  uint32(time.Now().Unix()),
						Mbox:    "hostmaster.consul.",
						Refresh: 1,
						Expire:  3,
						Retry:   2,
						Minttl:  4,
					},
				},
			},
		},
		{
			name: "PTR lookup for invalid subdomain",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.blah.arpa",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNameError,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.blah.arpa.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "consul.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    4,
						},
						Ns:      "ns.consul.",
						Serial:  uint32(time.Now().Unix()),
						Mbox:    "hostmaster.consul.",
						Refresh: 1,
						Expire:  3,
						Retry:   2,
						Minttl:  4,
					},
				},
			},
		},
		{
			name: "[ENT] PTR Lookup for node w/ peer name in default partition, query type is ANY",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node:    &discovery.Location{Name: "foo", Address: "1.2.3.4"},
						Type:    discovery.ResultTypeNode,
						Service: &discovery.Location{Name: "foo-web", Address: "foo"},
						Tenancy: discovery.ResultTenancy{
							Datacenter: "dc2",
							PeerName:   "peer1",
							Partition:  "default",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchRecordsByIp", mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(net.IP)

						require.NotNil(t, req)
						require.Equal(t, "1.2.3.4", req.String())
					})
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.PTR{
						Hdr: dns.RR_Header{
							Name:   "4.3.2.1.in-addr.arpa.",
							Rrtype: dns.TypePTR,
							Class:  dns.ClassINET,
						},
						Ptr: "foo.node.peer1.peer.default.ap.consul.",
					},
				},
			},
		},
		{
			name: "[ENT] PTR Lookup for service in default namespace, query type is PTR",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node:    &discovery.Location{Name: "foo", Address: "1.2.3.4"},
						Type:    discovery.ResultTypeService,
						Service: &discovery.Location{Name: "foo", Address: "foo"},
						Tenancy: discovery.ResultTenancy{
							Datacenter: "dc2",
							Namespace:  "default",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchRecordsByIp", mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(net.IP)

						require.NotNil(t, req)
						require.Equal(t, "1.2.3.4", req.String())
					})
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa.",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.PTR{
						Hdr: dns.RR_Header{
							Name:   "4.3.2.1.in-addr.arpa.",
							Rrtype: dns.TypePTR,
							Class:  dns.ClassINET,
						},
						Ptr: "foo.service.default.dc2.consul.",
					},
				},
			},
		},
		{
			name: "[ENT] PTR Lookup for service in a non-default namespace, query type is PTR",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node:    &discovery.Location{Name: "foo-node", Address: "1.2.3.4"},
						Type:    discovery.ResultTypeService,
						Service: &discovery.Location{Name: "foo", Address: "foo"},
						Tenancy: discovery.ResultTenancy{
							Datacenter: "dc2",
							Namespace:  "bar",
							Partition:  "baz",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchRecordsByIp", mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(net.IP)

						require.NotNil(t, req)
						require.Equal(t, "1.2.3.4", req.String())
					})
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa.",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.PTR{
						Hdr: dns.RR_Header{
							Name:   "4.3.2.1.in-addr.arpa.",
							Rrtype: dns.TypePTR,
							Class:  dns.ClassINET,
						},
						Ptr: "foo.service.bar.dc2.consul.",
					},
				},
			},
		},
		{
			name: "[CE] PTR Lookup for node w/ peer name, query type is ANY",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node:    &discovery.Location{Name: "foo", Address: "1.2.3.4"},
						Type:    discovery.ResultTypeNode,
						Service: &discovery.Location{Name: "foo", Address: "foo"},
						Tenancy: discovery.ResultTenancy{
							Datacenter: "dc2",
							PeerName:   "peer1",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchRecordsByIp", mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(net.IP)

						require.NotNil(t, req)
						require.Equal(t, "1.2.3.4", req.String())
					})
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.PTR{
						Hdr: dns.RR_Header{
							Name:   "4.3.2.1.in-addr.arpa.",
							Rrtype: dns.TypePTR,
							Class:  dns.ClassINET,
						},
						Ptr: "foo.node.peer1.peer.consul.",
					},
				},
			},
		},
		{
			name: "[CE] PTR Lookup for service, query type is PTR",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node:    &discovery.Location{Name: "foo", Address: "1.2.3.4"},
						Service: &discovery.Location{Name: "foo", Address: "foo"},
						Type:    discovery.ResultTypeService,
						Tenancy: discovery.ResultTenancy{
							Datacenter: "dc2",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchRecordsByIp", mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(net.IP)

						require.NotNil(t, req)
						require.Equal(t, "1.2.3.4", req.String())
					})
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "4.3.2.1.in-addr.arpa.",
						Qtype:  dns.TypePTR,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.PTR{
						Hdr: dns.RR_Header{
							Name:   "4.3.2.1.in-addr.arpa.",
							Rrtype: dns.TypePTR,
							Class:  dns.ClassINET,
						},
						Ptr: "foo.service.dc2.consul.",
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
