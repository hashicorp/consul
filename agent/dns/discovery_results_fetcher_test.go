// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/discovery"
)

// testCaseBuildQueryFromDNSMessage is a test case for the buildQueryFromDNSMessage function.
type testCaseBuildQueryFromDNSMessage struct {
	name           string
	request        *dns.Msg
	requestContext *Context
	expectedQuery  *discovery.Query
}

// Test_buildQueryFromDNSMessage tests the buildQueryFromDNSMessage function.
func Test_buildQueryFromDNSMessage(t *testing.T) {

	testCases := []testCaseBuildQueryFromDNSMessage{
		// virtual ip queries
		{
			name: "test A 'virtual.' query",
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
					Name:    "db",
					Tenancy: discovery.QueryTenancy{},
				},
			},
		},
		{
			name: "test A 'virtual.' with kitchen sink labels",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "db.virtual.banana.ns.orange.ap.foo.peer.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeVirtual,
				QueryPayload: discovery.QueryPayload{
					Name: "db",
					Tenancy: discovery.QueryTenancy{
						Peer:      "foo",
						Namespace: "banana",
						Partition: "orange",
					},
				},
			},
		},
		{
			name: "test A 'virtual.' with implicit peer",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "db.virtual.foo.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeVirtual,
				QueryPayload: discovery.QueryPayload{
					Name: "db",
					Tenancy: discovery.QueryTenancy{
						Peer: "foo",
					},
				},
			},
		},
		{
			name: "test A 'virtual.' with implicit peer and namespace query",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "db.virtual.frontend.foo.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeVirtual,
				QueryPayload: discovery.QueryPayload{
					Name: "db",
					Tenancy: discovery.QueryTenancy{
						Namespace: "frontend",
						Peer:      "foo",
					},
				},
			},
		},
		{
			name: "test A 'workload.'",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.workload.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeWorkload,
				QueryPayload: discovery.QueryPayload{
					Name:    "foo",
					Tenancy: discovery.QueryTenancy{},
				},
			},
		},
		{
			name: "test A 'workload.' with all possible labels",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "api.port.foo.workload.banana.ns.orange.ap.apple.peer.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			requestContext: &Context{
				DefaultPartition: "default-partition",
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeWorkload,
				QueryPayload: discovery.QueryPayload{
					Name:     "foo",
					PortName: "api",
					Tenancy: discovery.QueryTenancy{
						Namespace: "banana",
						Partition: "orange",
						Peer:      "apple",
					},
				},
			},
		},
		{
			name: "test sameness group with all possible labels",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.service.apple.sg.banana.ns.orange.ap.consul", // "intentionally missing the trailing dot"
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			requestContext: &Context{
				DefaultPartition: "default-partition",
			},
			expectedQuery: &discovery.Query{
				QueryType: discovery.QueryTypeService,
				QueryPayload: discovery.QueryPayload{
					Name: "foo",
					Tenancy: discovery.QueryTenancy{
						Namespace:     "banana",
						Partition:     "orange",
						SamenessGroup: "apple",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			context := tc.requestContext
			if context == nil {
				context = &Context{}
			}
			query, err := buildQueryFromDNSMessage(tc.request, *context, "consul.", ".", nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedQuery, query)
		})
	}
}
