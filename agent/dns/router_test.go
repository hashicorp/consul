// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"net"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/discovery"
)

// TODO (v2-dns)

// Test Parameters
// 	1. Domain vs AltDomain vs non-consul Main domain
//  2. Reload the configuration (e.g. SOA)
//  3. Something to check the token makes it through to the data fetcher
// 4. Something case insensitive

func Test_HandleRequest(t *testing.T) {

	type testCase struct {
		name                        string
		agentConfig                 *config.RuntimeConfig // This will override the default test Router Config
		mockProcessorResponseByName []*discovery.Result   // These will be fed to the mock processor to be returned in order
		mockProcessorResponseByIP   []*discovery.Result
		mockProcessorError          error
		request                     *dns.Msg
		requestContext              *discovery.Context
		remoteAddress               net.Addr
		response                    *dns.Msg
	}

	testCases := []testCase{
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
	}

	run := func(t *testing.T, tc testCase) {
		cfg := buildDNSConfig(tc.agentConfig, tc.mockProcessorResponseByName, tc.mockProcessorResponseByIP, tc.mockProcessorError)

		router, err := NewRouter(cfg)
		require.NoError(t, err)

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

func buildDNSConfig(agentConfig *config.RuntimeConfig, _ []*discovery.Result, _ []*discovery.Result, _ error) Config {
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
		EntMeta:   nil,
		Logger:    hclog.NewNullLogger(),
		Processor: nil, // TODO (v2-dns): build this from a mock with the reponses loaded
		TokenFunc: func() string { return "" },
	}

	if agentConfig != nil {
		cfg.AgentConfig = agentConfig
	}

	return cfg
}
