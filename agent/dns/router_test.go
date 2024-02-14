// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/internal/dnsutil"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
)

type HandleTestCase struct {
	name                         string
	agentConfig                  *config.RuntimeConfig // This will override the default test Router Config
	configureDataFetcher         func(fetcher discovery.CatalogDataFetcher)
	validateAndNormalizeExpected bool
	configureRecursor            func(recursor dnsRecursor)
	mockProcessorError           error
	request                      *dns.Msg
	requestContext               *Context
	remoteAddress                net.Addr
	response                     *dns.Msg
}

var testSOA = &dns.SOA{
	Hdr: dns.RR_Header{
		Name:   "consul.",
		Rrtype: dns.TypeSOA,
		Class:  dns.ClassINET,
		Ttl:    4,
	},
	Ns:      "ns.consul.",
	Mbox:    "hostmaster.consul.",
	Serial:  uint32(time.Now().Unix()),
	Refresh: 1,
	Retry:   2,
	Expire:  3,
	Minttl:  4,
}

func Test_HandleRequest(t *testing.T) {
	testCases := []HandleTestCase{
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
				DNSRecursors:      []string{"8.8.8.8"},
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
				DNSRecursors:      []string{"8.8.8.8"},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
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
				DNSRecursors:      []string{"8.8.8.8"},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
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
				DNSRecursors:      []string{"8.8.8.8"},
				DNSUDPAnswerLimit: maxUDPAnswerLimit,
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
							}},
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
		// NS Queries
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
		// PTR Lookups
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
		// V2 Workload Lookup
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
						},
						Target: "foo.example.com.",
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.example.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
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
						},
						Target: "foo.example.com.",
					},
					// TODO (v2-dns): this next record is wrong per the RFC
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.example.com.",
							Rrtype: dns.TypeCNAME,
							Class:  dns.ClassINET,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		// V2 Services
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
							Name:   "foo.service.consul.", // TODO (v2-dns): verify this shouldn't include tenancy for workloads
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
					// TODO (v2-dns): This needs to be de-dupped
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
		// V1 Prepared Queries
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

func runHandleTestCases(t *testing.T, tc HandleTestCase) {
	cdf := discovery.NewMockCatalogDataFetcher(t)
	if tc.validateAndNormalizeExpected {
		cdf.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
		cdf.On("NormalizeRequest", mock.Anything).Return()
	}

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
		ctx = &Context{}
	}

	var remoteAddress net.Addr
	if tc.remoteAddress != nil {
		remoteAddress = tc.remoteAddress
	} else {
		remoteAddress = &net.UDPAddr{}
	}

	actual := router.HandleRequest(tc.request, *ctx, remoteAddress)
	require.Equal(t, tc.response, actual)
}

func TestRouterDynamicConfig_GetTTLForService(t *testing.T) {
	type testCase struct {
		name             string
		inputKey         string
		shouldMatch      bool
		expectedDuration time.Duration
	}

	testCases := []testCase{
		{
			name:             "strict match",
			inputKey:         "foo",
			shouldMatch:      true,
			expectedDuration: 1 * time.Second,
		},
		{
			name:             "wildcard match",
			inputKey:         "bar",
			shouldMatch:      true,
			expectedDuration: 2 * time.Second,
		},
		{
			name:             "wildcard match 2",
			inputKey:         "bart",
			shouldMatch:      true,
			expectedDuration: 2 * time.Second,
		},
		{
			name:             "no match",
			inputKey:         "homer",
			shouldMatch:      false,
			expectedDuration: 0 * time.Second,
		},
	}

	rtCfg := &config.RuntimeConfig{
		DNSServiceTTL: map[string]time.Duration{
			"foo":  1 * time.Second,
			"bar*": 2 * time.Second,
		},
	}
	cfg, err := getDynamicRouterConfig(rtCfg)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, ok := cfg.getTTLForService(tc.inputKey)
			require.Equal(t, tc.shouldMatch, ok)
			require.Equal(t, tc.expectedDuration, actual)
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
			DNSUDPAnswerLimit: maxUDPAnswerLimit,
		},
		EntMeta:   acl.EnterpriseMeta{},
		Logger:    hclog.NewNullLogger(),
		Processor: discovery.NewQueryProcessor(cdf),
		TokenFunc: func() string { return "" },
		TranslateServiceAddressFunc: func(dc string, address string, taggedAddresses map[string]structs.ServiceAddress, accept dnsutil.TranslateAddressAccept) string {
			return address
		},
		TranslateAddressFunc: func(dc string, addr string, taggedAddresses map[string]string, accept dnsutil.TranslateAddressAccept) string {
			return addr
		},
	}

	if agentConfig != nil {
		cfg.AgentConfig = agentConfig
	}

	return cfg
}

// TestDNS_BinaryTruncate tests the dnsBinaryTruncate function.
func TestDNS_BinaryTruncate(t *testing.T) {
	msgSrc := new(dns.Msg)
	msgSrc.Compress = true
	msgSrc.SetQuestion("redis.service.consul.", dns.TypeSRV)

	for i := 0; i < 5000; i++ {
		target := fmt.Sprintf("host-redis-%d-%d.test.acme.com.node.dc1.consul.", i/256, i%256)
		msgSrc.Answer = append(msgSrc.Answer, &dns.SRV{Hdr: dns.RR_Header{Name: "redis.service.consul.", Class: 1, Rrtype: dns.TypeSRV, Ttl: 0x3c}, Port: 0x4c57, Target: target})
		msgSrc.Extra = append(msgSrc.Extra, &dns.CNAME{Hdr: dns.RR_Header{Name: target, Class: 1, Rrtype: dns.TypeCNAME, Ttl: 0x3c}, Target: fmt.Sprintf("fx.168.%d.%d.", i/256, i%256)})
	}
	for _, compress := range []bool{true, false} {
		for idx, maxSize := range []int{12, 256, 512, 8192, 65535} {
			t.Run(fmt.Sprintf("binarySearch %d", maxSize), func(t *testing.T) {
				msg := new(dns.Msg)
				msgSrc.Compress = compress
				msgSrc.SetQuestion("redis.service.consul.", dns.TypeSRV)
				msg.Answer = msgSrc.Answer
				msg.Extra = msgSrc.Extra
				msg.Ns = msgSrc.Ns
				index := make(map[string]dns.RR, len(msg.Extra))
				indexRRs(msg.Extra, index)
				blen := dnsBinaryTruncate(msg, maxSize, index, true)
				msg.Answer = msg.Answer[:blen]
				syncExtra(index, msg)
				predicted := msg.Len()
				buf, err := msg.Pack()
				if err != nil {
					t.Error(err)
				}
				if predicted < len(buf) {
					t.Fatalf("Bug in DNS library: %d != %d", predicted, len(buf))
				}
				if len(buf) > maxSize || (idx != 0 && len(buf) < 16) {
					t.Fatalf("bad[%d]: %d > %d", idx, len(buf), maxSize)
				}
			})
		}
	}
}

// TestDNS_syncExtra tests the syncExtra function.
func TestDNS_syncExtra(t *testing.T) {
	resp := &dns.Msg{
		Answer: []dns.RR{
			// These two are on the same host so the redundant extra
			// records should get deduplicated.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1002,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			// This one isn't in the Consul domain so it will get a
			// CNAME and then an A record from the recursor.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1003,
				Target: "demo.consul.io.",
			},
			// This one isn't in the Consul domain and it will get
			// a CNAME and A record from a recursor that alters the
			// case of the name. This proves we look up in the index
			// in a case-insensitive way.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "insensitive.consul.io.",
			},
			// This is also a CNAME, but it'll be set up to loop to
			// make sure we don't crash.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "deadly.consul.io.",
			},
			// This is also a CNAME, but it won't have another record.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "nope.consul.io.",
			},
		},
		Extra: []dns.RR{
			// These should get deduplicated.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			// This is a normal CNAME followed by an A record but we
			// have flipped the order. The algorithm should emit them
			// in the opposite order.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			// These differ in case to test case insensitivity.
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			// This doesn't appear in the answer, so should get
			// dropped.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-186.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.186"),
			},
			// These two test edge cases with CNAME handling.
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}

	index := make(map[string]dns.RR)
	indexRRs(resp.Extra, index)
	syncExtra(index, resp)

	expected := &dns.Msg{
		Answer: resp.Answer,
		Extra: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

// getUint32Ptr return the pointer of an uint32 literal
func getUint32Ptr(i uint32) *uint32 {
	return &i
}
