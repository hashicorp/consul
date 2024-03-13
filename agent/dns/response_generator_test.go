// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestDNSResponseGenerator_generateResponseFromError(t *testing.T) {
	testCases := []struct {
		name             string
		opts             *generateResponseFromErrorOpts
		expectedResponse *dns.Msg
	}{
		{
			name: "error is nil returns server failure",
			opts: &generateResponseFromErrorOpts{
				req:    &dns.Msg{},
				logger: testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: nil,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: false,
					Rcode:         dns.RcodeServerFailure,
				},
			},
		},
		{
			name: "error is invalid question returns name error",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "invalid-question",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "invalid-question",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: errInvalidQuestion,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNameError,
				},
				Question: []dns.Question{
					{
						Name:   "invalid-question",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						Ns:     "ns.testdomain.",
						Mbox:   "hostmaster.testdomain.",
						Serial: uint32(time.Now().Unix()),
					},
				},
			},
		},
		{
			name: "error is name not found returns name error",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "invalid-name",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "invalid-name",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: errNameNotFound,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNameError,
				},
				Question: []dns.Question{
					{
						Name:   "invalid-name",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						Ns:     "ns.testdomain.",
						Mbox:   "hostmaster.testdomain.",
						Serial: uint32(time.Now().Unix()),
					},
				},
			},
		},
		{
			name: "error is not implemented returns not implemented error",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "some-question",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "some-question",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: errNotImplemented,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNotImplemented,
				},
				Question: []dns.Question{
					{
						Name:   "some-question",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						Ns:     "ns.testdomain.",
						Mbox:   "hostmaster.testdomain.",
						Serial: uint32(time.Now().Unix()),
					},
				},
			},
		},
		{
			name: "error is not supported returns name error",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "some-question",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "some-question",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: discovery.ErrNotSupported,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNameError,
				},
				Question: []dns.Question{
					{
						Name:   "some-question",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						Ns:     "ns.testdomain.",
						Mbox:   "hostmaster.testdomain.",
						Serial: uint32(time.Now().Unix()),
					},
				},
			},
		},
		{
			name: "error is not found returns name error",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "some-question",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "some-question",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: discovery.ErrNotFound,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNameError,
				},
				Question: []dns.Question{
					{
						Name:   "some-question",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						Ns:     "ns.testdomain.",
						Mbox:   "hostmaster.testdomain.",
						Serial: uint32(time.Now().Unix()),
					},
				},
			},
		},
		{
			name: "error is no data returns success with soa",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "some-question",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "some-question",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: discovery.ErrNoData,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
				Question: []dns.Question{
					{
						Name:   "some-question",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						Ns:     "ns.testdomain.",
						Mbox:   "hostmaster.testdomain.",
						Serial: uint32(time.Now().Unix()),
					},
				},
			},
		},
		{
			name: "error is no path to datacenter returns name error",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "some-question",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "some-question",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: discovery.ErrNoPathToDatacenter,
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: true,
					Rcode:         dns.RcodeNameError,
				},
				Question: []dns.Question{
					{
						Name:   "some-question",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: []dns.RR{
					&dns.SOA{
						Hdr: dns.RR_Header{
							Name:   "testdomain.",
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    0,
						},
						Ns:     "ns.testdomain.",
						Mbox:   "hostmaster.testdomain.",
						Serial: uint32(time.Now().Unix()),
					},
				},
			},
		},
		{
			name: "error is something else returns server failure error",
			opts: &generateResponseFromErrorOpts{
				req: &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "some-question",
							Qtype:  dns.TypeSRV,
							Qclass: dns.ClassANY,
						},
					},
				},
				qName:          "some-question",
				responseDomain: "testdomain.",
				logger:         testutil.Logger(t),
				configCtx: &RouterDynamicConfig{
					DisableCompression: true,
				},
				err: errors.New("KABOOM"),
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: false,
					Rcode:         dns.RcodeServerFailure,
				},
				Question: []dns.Question{
					{
						Name:   "some-question",
						Qtype:  dns.TypeSRV,
						Qclass: dns.ClassANY,
					},
				},
				Ns: nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.req.IsEdns0()
			actualResponse := dnsResponseGenerator{}.generateResponseFromError(tc.opts)
			require.Equal(t, tc.expectedResponse, actualResponse)
		})
	}
}

func TestDNSResponseGenerator_setEDNS(t *testing.T) {
	testCases := []struct {
		name             string
		req              *dns.Msg
		response         *dns.Msg
		ecsGlobal        bool
		expectedResponse *dns.Msg
	}{
		{
			name: "request is not edns0, response is not edns0",
			req: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Extra: []dns.RR{
					&dns.OPT{
						Hdr: dns.RR_Header{
							Name:   ".",
							Rrtype: dns.TypeOPT,
							Class:  4096,
							Ttl:    0,
						},
						Option: []dns.EDNS0{
							&dns.EDNS0_SUBNET{
								Code:          1,
								Family:        2,
								SourceNetmask: 3,
								SourceScope:   4,
								Address:       net.ParseIP("255.255.255.255"),
							},
						},
					},
				},
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Extra: []dns.RR{
					&dns.OPT{
						Hdr: dns.RR_Header{
							Name:   ".",
							Rrtype: dns.TypeOPT,
							Class:  4096,
							Ttl:    0,
						},
						Option: []dns.EDNS0{
							&dns.EDNS0_SUBNET{
								Code:          8,
								Family:        2,
								SourceNetmask: 3,
								SourceScope:   3,
								Address:       net.ParseIP("255.255.255.255"),
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dnsResponseGenerator{}.setEDNS(tc.req, tc.response, tc.ecsGlobal)
			require.Equal(t, tc.expectedResponse, tc.response)
		})
	}
}

func TestDNSResponseGenerator_trimDNSResponse(t *testing.T) {
	testCases := []struct {
		name             string
		req              *dns.Msg
		response         *dns.Msg
		cfg              *RouterDynamicConfig
		remoteAddress    net.Addr
		expectedResponse *dns.Msg
	}{
		{
			name: "network is udp, enable truncate is true, answer count of 1 is less/equal than configured max f 1, response is not trimmed",
			req: &dns.Msg{
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
			cfg: &RouterDynamicConfig{
				UDPAnswerLimit: 1,
			},
			remoteAddress: &net.UDPAddr{
				IP: net.ParseIP("127.0.0.1"),
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
					Rcode:  dns.RcodeSuccess,
				},
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
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Rcode: dns.RcodeSuccess,
				},
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
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "network is udp, enable truncate is true, answer count of 2 is greater than configure UDP max f 2, response is trimmed",
			req: &dns.Msg{
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
			cfg: &RouterDynamicConfig{
				UDPAnswerLimit: 1,
			},
			remoteAddress: &net.UDPAddr{
				IP: net.ParseIP("127.0.0.1"),
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
					Rcode:  dns.RcodeSuccess,
				},
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
							Name:   "foo1.query.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo2.query.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("2.2.3.4"),
					},
				},
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Rcode: dns.RcodeSuccess,
				},
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
							Name:   "foo1.query.consul.",
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
			name: "network is tcp, enable truncate is true, answer is less than 64k limit, response is not trimmed",
			req: &dns.Msg{
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
			cfg: &RouterDynamicConfig{},
			remoteAddress: &net.TCPAddr{
				IP: net.ParseIP("127.0.0.1"),
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
					Rcode:  dns.RcodeSuccess,
				},
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
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
			expectedResponse: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Rcode: dns.RcodeSuccess,
				},
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
			logger := testutil.Logger(t)
			dnsResponseGenerator{}.trimDNSResponse(tc.cfg, tc.remoteAddress, tc.req, tc.response, logger)
			require.Equal(t, tc.expectedResponse, tc.response)
		})

	}
}
