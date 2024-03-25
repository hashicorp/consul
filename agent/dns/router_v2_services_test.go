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
	"github.com/hashicorp/consul/internal/resource"
)

func Test_HandleRequest_V2Services(t *testing.T) {
	testCases := []HandleTestCase{
		{
			name: "A/AAAA Query a service and return multiple A records",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node: &discovery.Location{Name: "foo-1", Address: "10.0.0.1"},
						Type: discovery.ResultTypeWorkload,
						Tenancy: discovery.ResultTenancy{
							Namespace: resource.DefaultNamespaceName,
							Partition: resource.DefaultPartitionName,
						},
						Ports: []discovery.Port{
							{
								Name:   "api",
								Number: 5678,
							},
							// Intentionally not in the mesh
						},
						DNS: discovery.DNSConfig{
							Weight: 2,
						},
					},
					{
						Node: &discovery.Location{Name: "foo-2", Address: "10.0.0.2"},
						Type: discovery.ResultTypeWorkload,
						Tenancy: discovery.ResultTenancy{
							Namespace: resource.DefaultNamespaceName,
							Partition: resource.DefaultPartitionName,
						},
						Ports: []discovery.Port{
							{
								Name:   "api",
								Number: 5678,
							},
							{
								Name:   "mesh",
								Number: 21000,
							},
						},
						DNS: discovery.DNSConfig{
							Weight: 3,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, "foo", req.Name)
						require.Empty(t, req.PortName)
						require.Equal(t, discovery.LookupTypeService, reqType)
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
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						A: net.ParseIP("10.0.0.1"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						A: net.ParseIP("10.0.0.2"),
					},
				},
			},
		},
		{
			name: "SRV Query with a multi-port service return multiple SRV records",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node: &discovery.Location{Name: "foo-1", Address: "10.0.0.1"},
						Type: discovery.ResultTypeWorkload,
						Tenancy: discovery.ResultTenancy{
							Namespace: resource.DefaultNamespaceName,
							Partition: resource.DefaultPartitionName,
						},
						Ports: []discovery.Port{
							{
								Name:   "api",
								Number: 5678,
							},
							// Intentionally not in the mesh
						},
						DNS: discovery.DNSConfig{
							Weight: 2,
						},
					},
					{
						Node: &discovery.Location{Name: "foo-2", Address: "10.0.0.2"},
						Type: discovery.ResultTypeWorkload,
						Tenancy: discovery.ResultTenancy{
							Namespace: resource.DefaultNamespaceName,
							Partition: resource.DefaultPartitionName,
						},
						Ports: []discovery.Port{
							{
								Name:   "api",
								Number: 5678,
							},
							{
								Name:   "mesh",
								Number: 21000,
							},
						},
						DNS: discovery.DNSConfig{
							Weight: 3,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, "foo", req.Name)
						require.Empty(t, req.PortName)
						require.Equal(t, discovery.LookupTypeService, reqType)
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
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   2,
						Priority: 1,
						Port:     5678,
						Target:   "api.port.foo-1.workload.default.ns.default.ap.consul.",
					},
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   3,
						Priority: 1,
						Port:     5678,
						Target:   "api.port.foo-2.workload.default.ns.default.ap.consul.",
					},
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   3,
						Priority: 1,
						Port:     21000,
						Target:   "mesh.port.foo-2.workload.default.ns.default.ap.consul.",
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "api.port.foo-1.workload.default.ns.default.ap.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						A: net.ParseIP("10.0.0.1"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "api.port.foo-2.workload.default.ns.default.ap.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						A: net.ParseIP("10.0.0.2"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "mesh.port.foo-2.workload.default.ns.default.ap.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						A: net.ParseIP("10.0.0.2"),
					},
				},
			},
		},
		{
			name: "SRV Query with a multi-port service where the client requests a specific port, returns SRV and A records",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "mesh.port.foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node: &discovery.Location{Name: "foo-2", Address: "10.0.0.2"},
						Type: discovery.ResultTypeWorkload,
						Tenancy: discovery.ResultTenancy{
							Namespace: resource.DefaultNamespaceName,
							Partition: resource.DefaultPartitionName,
						},
						Ports: []discovery.Port{
							{
								Name:   "mesh",
								Number: 21000,
							},
						},
						DNS: discovery.DNSConfig{
							Weight: 3,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, "foo", req.Name)
						require.Equal(t, "mesh", req.PortName)
						require.Equal(t, discovery.LookupTypeService, reqType)
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
						Name:   "mesh.port.foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "mesh.port.foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   3,
						Priority: 1,
						Port:     21000,
						Target:   "mesh.port.foo-2.workload.default.ns.default.ap.consul.",
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "mesh.port.foo-2.workload.default.ns.default.ap.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						A: net.ParseIP("10.0.0.2"),
					},
				},
			},
		},
		{
			name: "SRV Query with a multi-port service that has workloads w/ hostnames (no recursors)",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node: &discovery.Location{Name: "foo-1", Address: "foo-1.example.com"},
						Type: discovery.ResultTypeWorkload,
						Tenancy: discovery.ResultTenancy{
							Namespace: resource.DefaultNamespaceName,
							Partition: resource.DefaultPartitionName,
						},
						Ports: []discovery.Port{
							{
								Name:   "api",
								Number: 5678,
							},
							{
								Name:   "web",
								Number: 8080,
							},
						},
						DNS: discovery.DNSConfig{
							Weight: 2,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, "foo", req.Name)
						require.Empty(t, req.PortName)
						require.Equal(t, discovery.LookupTypeService, reqType)
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
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   2,
						Priority: 1,
						Port:     5678,
						Target:   "foo-1.example.com.",
					},
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   2,
						Priority: 1,
						Port:     8080,
						Target:   "foo-1.example.com.",
					},
				},
			},
		},
		{
			name: "SRV Query with a multi-port service that has workloads w/ hostnames (no recursor)",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				results := []*discovery.Result{
					{
						Node: &discovery.Location{Name: "foo-1", Address: "foo-1.example.com"},
						Type: discovery.ResultTypeWorkload,
						Tenancy: discovery.ResultTenancy{
							Namespace: resource.DefaultNamespaceName,
							Partition: resource.DefaultPartitionName,
						},
						Ports: []discovery.Port{
							{
								Name:   "api",
								Number: 5678,
							},
							{
								Name:   "web",
								Number: 8080,
							},
						},
						DNS: discovery.DNSConfig{
							Weight: 2,
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(results, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, "foo", req.Name)
						require.Empty(t, req.PortName)
						require.Equal(t, discovery.LookupTypeService, reqType)
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
							Name:   "foo-1.example.com.",
							Qtype:  dns.TypeA,
							Qclass: dns.ClassINET,
						},
					},
					Answer: []dns.RR{
						&dns.A{
							Hdr: dns.RR_Header{
								Name:   "foo-1.example.com.",
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
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   2,
						Priority: 1,
						Port:     5678,
						Target:   "foo-1.example.com.",
					},
					&dns.SRV{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeSRV,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						Weight:   2,
						Priority: 1,
						Port:     8080,
						Target:   "foo-1.example.com.",
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo-1.example.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
						},
						A: net.ParseIP("1.2.3.4"),
					},
					// TODO (v2-dns): This needs to be de-duplicated (NET-8064)
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo-1.example.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    uint32(123),
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
