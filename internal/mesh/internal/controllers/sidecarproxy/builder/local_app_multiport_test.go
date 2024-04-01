// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestBuildLocalApp_Multiport(t *testing.T) {
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		cases := map[string]struct {
			workload *pbcatalog.Workload
		}{
			"source/multiport-l7-single-workload-address-without-ports": {
				workload: &pbcatalog.Workload{
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host: "10.0.0.1",
						},
					},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
						"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
						"grpc-port":  {Port: 9091, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
						"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
			},
			"source/multiport-l7-multiple-workload-addresses-without-ports": {
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
						"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
						"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
						"grpc-port":  {Port: 9091, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
						"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
			},
			"source/multiport-l7-multiple-workload-addresses-with-specific-ports": {
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
						"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
						"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
						"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
			},
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
			t.Run(resourcetest.AppendTenancyInfoSubtest(t.Name(), name, tenancy), func(t *testing.T) {
				proxyTmpl := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", false, nil).
					BuildLocalApp(c.workload, nil).
					Build()

				// sort routers because of test flakes where order was flip flopping.
				actualRouters := proxyTmpl.ProxyState.Listeners[0].Routers
				sort.Slice(actualRouters, func(i, j int) bool {
					return actualRouters[i].String() < actualRouters[j].String()
				})

				actual := protoToJSON(t, proxyTmpl)
				expected := JSONToProxyTemplate(t, golden.GetBytes(t, []byte(actual), name+"-"+tenancy.Partition+"-"+tenancy.Namespace+".golden"))

				// sort routers on listener from golden file
				expectedRouters := expected.ProxyState.Listeners[0].Routers
				sort.Slice(expectedRouters, func(i, j int) bool {
					return expectedRouters[i].String() < expectedRouters[j].String()
				})

				// convert back to json after sorting so that test output does not contain extraneous fields.
				require.Equal(t, protoToJSON(t, expected), protoToJSON(t, proxyTmpl))
			})
		}
	}, t)
}
