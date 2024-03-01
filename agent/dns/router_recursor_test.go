// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"errors"
	"github.com/hashicorp/consul/agent/config"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"net"
	"testing"
)

func Test_HandleRequest_recursor(t *testing.T) {
	testCases := []HandleTestCase{
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runHandleTestCases(t, tc)
		})
	}
}
