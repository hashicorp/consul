// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/agent/structs"
)

// TODO (v2-dns)

// TBD Test Cases
//  1. Reload the configuration (e.g. SOA)
//  2. Something to check the token makes it through to the data fetcher
//  3. Something case-insensitive
//  4. Test the edns settings.

func Test_HandleRequest(t *testing.T) {
	type testCase struct {
		name                 string
		agentConfig          *config.RuntimeConfig // This will override the default test Router Config
		configureDataFetcher func(fetcher discovery.CatalogDataFetcher)
		configureRecursor    func(recursor dnsRecursor)
		mockProcessorError   error
		request              *dns.Msg
		requestContext       *discovery.Context
		remoteAddress        net.Addr
		response             *dns.Msg
	}

	testCases := []testCase{
		// recursor queries
		{
			name: "recursors not configured, non-matching domain",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "google.com",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			// configureRecursor: call not expected.
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:   dns.OpcodeQuery,
					Response: true,
					Rcode:    dns.RcodeRefused,
				},
				Question: []dns.Question{
					{
						Name:   "google.com.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
		},
		{
			name: "recursors configured, matching domain",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "google.com",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			agentConfig: &config.RuntimeConfig{
				DNSRecursors: []string{"8.8.8.8"},
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
							Name:   "google.com.",
							Qtype:  dns.TypeA,
							Qclass: dns.ClassINET,
						},
					},
					Answer: []dns.RR{
						&dns.A{
							Hdr: dns.RR_Header{
								Name:   "google.com.",
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
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
				Question: []dns.Question{
					{
						Name:   "google.com.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "google.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "recursors configured, no matching domain",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "google.com",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			agentConfig: &config.RuntimeConfig{
				DNSRecursors: []string{"8.8.8.8"},
			},
			configureRecursor: func(recursor dnsRecursor) {
				recursor.(*mockDnsRecursor).On("handle", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errRecursionFailed)
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:             dns.OpcodeQuery,
					Response:           true,
					Authoritative:      false,
					Rcode:              dns.RcodeServerFailure,
					RecursionAvailable: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "google.com.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
		},
		{
			name: "recursors configured, unhandled error calling recursors",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "google.com",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			agentConfig: &config.RuntimeConfig{
				DNSRecursors: []string{"8.8.8.8"},
			},
			configureRecursor: func(recursor dnsRecursor) {
				err := errors.New("ahhhhh!!!!")
				recursor.(*mockDnsRecursor).On("handle", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, err)
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:             dns.OpcodeQuery,
					Response:           true,
					Authoritative:      false,
					Rcode:              dns.RcodeServerFailure,
					RecursionAvailable: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "google.com.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
		},
		{
			name: "recursors configured, the root domain is handled by the recursor",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   ".",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			agentConfig: &config.RuntimeConfig{
				DNSRecursors: []string{"8.8.8.8"},
			},
			configureRecursor: func(recursor dnsRecursor) {
				// this response is modeled after `dig .`
				resp := &dns.Msg{
					MsgHdr: dns.MsgHdr{
						Opcode:        dns.OpcodeQuery,
						Response:      true,
						Authoritative: true,
						Rcode:         dns.RcodeSuccess,
					},
					Question: []dns.Question{
						{
							Name:   ".",
							Qtype:  dns.TypeA,
							Qclass: dns.ClassINET,
						},
					},
					Answer: []dns.RR{
						&dns.SOA{
							Hdr: dns.RR_Header{
								Name:   ".",
								Rrtype: dns.TypeSOA,
								Class:  dns.ClassINET,
								Ttl:    86391,
							},
							Ns:      "a.root-servers.net.",
							Serial:  2024012200,
							Mbox:    "nstld.verisign-grs.com.",
							Refresh: 1800,
							Retry:   900,
							Expire:  604800,
							Minttl:  86400,
						},
					},
				}
				recursor.(*mockDnsRecursor).On("handle",
					mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
				Question: []dns.Question{
					{
						Name:   ".",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   ".",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    86391,
						},
						Ns:      "a.root-servers.net.",
						Serial:  2024012200,
						Mbox:    "nstld.verisign-grs.com.",
						Refresh: 1800,
						Retry:   900,
						Expire:  604800,
						Minttl:  86400,
					},
				},
			},
		},
		// addr queries
		{
			name: "test A 'addr.' query, ipv4 response",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "c000020a.addr.dc1.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
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
						Name:   "c000020a.addr.dc1.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "c000020a.addr.dc1.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("192.0.2.10"),
					},
				},
			},
		},
		{
			name: "test AAAA 'addr.' query, ipv4 response",
			// Since we asked for an AAAA record, the A record that resolves from the address is attached as an extra
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "c000020a.addr.dc1.consul",
						Qtype:  dns.TypeAAAA,
						Qclass: dns.ClassINET,
					},
				},
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
						Name:   "c000020a.addr.dc1.consul.",
						Qtype:  dns.TypeAAAA,
						Qclass: dns.ClassINET,
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "c000020a.addr.dc1.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("192.0.2.10"),
					},
				},
			},
		},
		{
			name: "test SRV 'addr.' query, ipv4 response",
			// Since we asked for a SRV record, the A record that resolves from the address is attached as an extra
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "c000020a.addr.dc1.consul",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
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
						Name:   "c000020a.addr.dc1.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "c000020a.addr.dc1.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("192.0.2.10"),
					},
				},
			},
		},
		{
			name: "test ANY 'addr.' query, ipv4 response",
			// The response to ANY should look the same as the A response
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "c000020a.addr.dc1.consul",
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
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "c000020a.addr.dc1.consul.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "c000020a.addr.dc1.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("192.0.2.10"),
					},
				},
			},
		},
		{
			name: "test AAAA 'addr.' query, ipv6 response",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul",
						Qtype:  dns.TypeAAAA,
						Qclass: dns.ClassINET,
					},
				},
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
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
						Qtype:  dns.TypeAAAA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						AAAA: net.ParseIP("2001:db8:1:2:cafe::1337"),
					},
				},
			},
		},
		{
			name: "test A 'addr.' query, ipv6 response",
			// Since we asked for an A record, the AAAA record that resolves from the address is attached as an extra
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
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
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Extra: []dns.RR{
					&dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						AAAA: net.ParseIP("2001:db8:1:2:cafe::1337"),
					},
				},
			},
		},
		{
			name: "test SRV 'addr.' query, ipv6 response",
			// Since we asked for an SRV record, the AAAA record that resolves from the address is attached as an extra
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
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
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassINET,
					},
				},
				Extra: []dns.RR{
					&dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						AAAA: net.ParseIP("2001:db8:1:2:cafe::1337"),
					},
				},
			},
		},
		{
			name: "test ANY 'addr.' query, ipv6 response",
			// The response to ANY should look the same as the AAAA response
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul",
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
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
						Qtype:  dns.TypeANY,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   "20010db800010002cafe000000001337.addr.dc1.consul.",
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						AAAA: net.ParseIP("2001:db8:1:2:cafe::1337"),
					},
				},
			},
		},
		{
			name: "test malformed 'addr.' query",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "c000.addr.dc1.consul", // too short
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Rcode:         dns.RcodeNameError, // NXDOMAIN
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "c000.addr.dc1.consul.",
						Qtype:  dns.TypeA,
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
		// virtual ip queries - we will test just the A record, since the
		// AAAA and SRV records are handled the same way and the complete
		// set of addr tests above cover the rest of the cases.
		{
			name: "test A 'virtual.' query, ipv4 response",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "c000020a.virtual.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).On("FetchVirtualIP",
					mock.Anything, mock.Anything).Return(&discovery.Result{
					Address: "240.0.0.2",
					Type:    discovery.ResultTypeVirtual,
				}, nil)
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
						Name:   "c000020a.virtual.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "c000020a.virtual.consul.",
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
					Address: "2001:db8:1:2:cafe::1337",
					Type:    discovery.ResultTypeVirtual,
				}, nil)
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
		// SOA Queries
		{
			name: "vanilla SOA query",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "consul.",
						Qtype:  dns.TypeSOA,
						Qclass: dns.ClassINET,
					},
				},
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return([]*discovery.Result{
						{
							Address: "1.2.3.4",
							Type:    discovery.ResultTypeWorkload,
							Target:  "server-one", // This would correlate to the workload name
						},
						{
							Address: "4.5.6.7",
							Type:    discovery.ResultTypeWorkload,
							Target:  "server-two", // This would correlate to the workload name
						},
					}, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, discovery.LookupTypeService, reqType)
						require.Equal(t, structs.ConsulServiceName, req.Name)
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
						Name:   "consul.",
						Qtype:  dns.TypeSOA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
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
				Ns: []dns.RR{
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "consul.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-one.workload.consul.",
					},
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "consul.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-two.workload.consul.",
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-one.workload.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-two.workload.consul.",
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
			name: "SOA query against alternate domain",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "testdomain.",
						Qtype:  dns.TypeSOA,
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
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return([]*discovery.Result{
						{
							Address: "1.2.3.4",
							Type:    discovery.ResultTypeWorkload,
							Target:  "server-one", // This would correlate to the workload name
						},
						{
							Address: "4.5.6.7",
							Type:    discovery.ResultTypeWorkload,
							Target:  "server-two", // This would correlate to the workload name
						},
					}, nil).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, discovery.LookupTypeService, reqType)
						require.Equal(t, structs.ConsulServiceName, req.Name)
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
						Name:   "testdomain.",
						Qtype:  dns.TypeSOA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.SOA{
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
						Expire:  3,
						Retry:   2,
						Minttl:  4,
					},
				},
				Ns: []dns.RR{
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-one.workload.testdomain.",
					},
					&dns.NS{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeNS,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						Ns: "server-two.workload.testdomain.",
					},
				},
				Extra: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-one.workload.testdomain.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "server-two.workload.testdomain.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("4.5.6.7"),
					},
				},
			},
		},
		// Service Lookup
		{
			name: "When no data is return from a query, send SOA",
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
				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, discovery.ErrNoData).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, discovery.LookupTypeService, reqType)
						require.Equal(t, "foo", req.Name)
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
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeA,
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
		// TODO (v2-dns): add a test to make sure only 3 records are returned
	}

	run := func(t *testing.T, tc testCase) {
		cdf := discovery.NewMockCatalogDataFetcher(t)
		if tc.configureDataFetcher != nil {
			tc.configureDataFetcher(cdf)
		}
		cfg := buildDNSConfig(tc.agentConfig, cdf, tc.mockProcessorError)

		router, err := NewRouter(cfg)
		require.NoError(t, err)

		// Replace the recursor with a mock and configure
		router.recursor = newMockDnsRecursor(t)
		if tc.configureRecursor != nil {
			tc.configureRecursor(router.recursor)
		}

		ctx := tc.requestContext
		if ctx == nil {
			ctx = &discovery.Context{}
		}
		actual := router.HandleRequest(tc.request, *ctx, tc.remoteAddress)
		require.Equal(t, tc.response, actual)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}

}

func buildDNSConfig(agentConfig *config.RuntimeConfig, cdf discovery.CatalogDataFetcher, _ error) Config {
	cfg := Config{
		AgentConfig: &config.RuntimeConfig{
			DNSDomain:  "consul",
			DNSNodeTTL: 123 * time.Second,
			DNSSOA: config.RuntimeSOAConfig{
				Refresh: 1,
				Retry:   2,
				Expire:  3,
				Minttl:  4,
			},
		},
		EntMeta:   acl.EnterpriseMeta{},
		Logger:    hclog.NewNullLogger(),
		Processor: discovery.NewQueryProcessor(cdf),
		TokenFunc: func() string { return "" },
	}

	if agentConfig != nil {
		cfg.AgentConfig = agentConfig
	}

	return cfg
}
