// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"sort"
	"testing"

	"github.com/hashicorp/consul/internal/testing/golden"

	"github.com/stretchr/testify/require"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
)

func TestBuildLocalApp_Multiport(t *testing.T) {
	cases := map[string]struct {
		workload *pbcatalog.Workload
	}{
		"source/multiport-l4-single-workload-address-without-ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.0.0.1",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
		"source/multiport-l4-multiple-workload-addresses-without-ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.0.0.1",
					},
					{
						Host: "10.0.0.2",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
		"source/multiport-l4-multiple-workload-addresses-with-specific-ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.0.0.1",
						Ports: []string{"admin-port"},
					},
					{
						Host:  "10.0.0.2",
						Ports: []string{"api-port"},
					},
					{
						Host:  "10.0.0.3",
						Ports: []string{"mesh"},
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
		"source/multiport-l4-workload-with-only-mesh-port": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.0.0.1",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", false, nil).
				BuildLocalApp(c.workload, nil).
				Build()

			// sort routers because of test flakes where order was flip flopping.
			actualRouters := proxyTmpl.ProxyState.Listeners[0].Routers
			sort.Slice(actualRouters, func(i, j int) bool {
				return actualRouters[i].String() < actualRouters[j].String()
			})

			actual := protoToJSON(t, proxyTmpl)
			expected := JSONToProxyTemplate(t, golden.GetBytes(t, actual, name+".golden"))

			// sort routers on listener from golden file
			expectedRouters := expected.ProxyState.Listeners[0].Routers
			sort.Slice(expectedRouters, func(i, j int) bool {
				return expectedRouters[i].String() < expectedRouters[j].String()
			})

			// convert back to json after sorting so that test output does not contain extraneous fields.
			require.Equal(t, protoToJSON(t, expected), protoToJSON(t, proxyTmpl))
		})
	}
}
