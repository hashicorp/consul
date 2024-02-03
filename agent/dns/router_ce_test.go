// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package dns

import (
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/discovery"
)

func getAdditionalTestCases(t *testing.T) []HandleTestCase {
	// PTR Lookups
	return []HandleTestCase{
		// PTR Lookups
		{
			name: "PTR Lookup for node w/ peer name, query type is ANY",
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
						Node:    &discovery.Location{Name: "foonode", Address: "1.2.3.4"},
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
			name: "PTR Lookup for service, query type is PTR",
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
						Node:    &discovery.Location{Name: "foonode", Address: "1.2.3.4"},
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
}
