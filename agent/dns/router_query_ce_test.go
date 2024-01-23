// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package dns

import (
	"github.com/miekg/dns"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/discovery"
)

func getBuildQueryFromDNSMessageTestCases() []testCaseBuildQueryFromDNSMessage {
	testCases := []testCaseBuildQueryFromDNSMessage{
		// virtual ip queries
		{
			name: "test A 'virtual.' query, ipv4 response",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "db.virtual.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeVirtual,
				QueryPayload: discovery.QueryPayload{
					Name:     "db",
					PortName: "",
					Tag:      "",
					Tenancy: discovery.QueryTenancy{
						EnterpriseMeta: acl.EnterpriseMeta{},
						SamenessGroup:  "",
						Peer:           "consul",
						Datacenter:     "",
					},
					DisableFailover: false,
				},
			},
		},
		{
			name: "test A 'virtual.' with peer query, ipv4 response",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "db.virtual.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeVirtual,
				QueryPayload: discovery.QueryPayload{
					Name:     "db",
					PortName: "",
					Tag:      "",
					Tenancy: discovery.QueryTenancy{
						EnterpriseMeta: acl.EnterpriseMeta{},
						SamenessGroup:  "",
						Peer:           "consul", // this gets set in the query building after ParseLocality processes.
						Datacenter:     "",
					},
					DisableFailover: false,
				},
			},
		},
	}

	return testCases
}
