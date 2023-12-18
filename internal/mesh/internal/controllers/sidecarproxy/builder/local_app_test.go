// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestBuildLocalApp(t *testing.T) {
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		cases := map[string]struct {
			workload     *pbcatalog.Workload
			ctp          *pbauth.ComputedTrafficPermissions
			defaultAllow bool
		}{
			"source/single-workload-address-without-ports": {
				workload: &pbcatalog.Workload{
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host: "10.0.0.1",
						},
					},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"tcp":   {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
						"http":  {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
						"http2": {Port: 8082, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
						"grpc":  {Port: 8083, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
						"mesh":  {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
			},
			"source/multiple-workload-addresses-without-ports": {
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
						"tcp":   {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
						"http":  {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
						"http2": {Port: 8082, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
						"grpc":  {Port: 8083, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
						"mesh":  {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
			},
			"source/multiple-workload-addresses-with-specific-ports": {
				workload: &pbcatalog.Workload{
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host:  "127.0.0.1",
							Ports: []string{"tcp", "grpc", "mesh"},
						},
						{
							Host:  "10.0.0.2",
							Ports: []string{"http", "http2", "mesh"},
						},
					},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"tcp":   {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
						"http":  {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
						"http2": {Port: 8082, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
						"grpc":  {Port: 8083, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
						"mesh":  {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
				ctp: &pbauth.ComputedTrafficPermissions{
					AllowPermissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "foo",
									Namespace:    "default",
									Partition:    "default",
								},
							},
						},
					},
				},
				defaultAllow: true,
			},
		}

		for name, c := range cases {
			t.Run(resourcetest.AppendTenancyInfoSubtest(t.Name(), name, tenancy), func(t *testing.T) {
				proxyTmpl := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", true, nil).
					BuildLocalApp(c.workload, nil).
					Build()

				// sort routers because of test flakes where order was flip flopping.
				actualRouters := proxyTmpl.ProxyState.Listeners[0].Routers
				sort.Slice(actualRouters, func(i, j int) bool {
					return actualRouters[i].String() < actualRouters[j].String()
				})

				actual := protoToJSON(t, proxyTmpl)
				expected := JSONToProxyTemplate(t, golden.GetBytes(t, actual, name+"-"+tenancy.Partition+"-"+tenancy.Namespace+".golden"))

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

func TestBuildLocalApp_WithProxyConfiguration(t *testing.T) {
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		cases := map[string]struct {
			workload *pbcatalog.Workload
			proxyCfg *pbmesh.ComputedProxyConfiguration
		}{
			"source/l7-expose-paths": {
				workload: &pbcatalog.Workload{
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host: "10.0.0.1",
						},
					},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
						"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				},
				proxyCfg: &pbmesh.ComputedProxyConfiguration{
					DynamicConfig: &pbmesh.DynamicConfig{
						ExposeConfig: &pbmesh.ExposeConfig{
							ExposePaths: []*pbmesh.ExposePath{
								{
									ListenerPort:  1234,
									Path:          "/health",
									LocalPathPort: 9090,
									Protocol:      pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP,
								},
								{
									ListenerPort:  1235,
									Path:          "GetHealth",
									LocalPathPort: 9091,
									Protocol:      pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP2,
								},
							},
						},
					},
				},
			},
			// source/local-and-inbound-connections shows that configuring LocalCOnnection
			// and InboundConnections in DynamicConfig will set fields on standard clusters and routes,
			// but will not set fields on exposed path clusters and routes.
			"source/local-and-inbound-connections": {
				workload: &pbcatalog.Workload{
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host: "10.0.0.1",
						},
					},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
						"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						"port3": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					},
				},
				proxyCfg: &pbmesh.ComputedProxyConfiguration{
					DynamicConfig: &pbmesh.DynamicConfig{
						LocalConnection: map[string]*pbmesh.ConnectionConfig{
							"port1": {
								ConnectTimeout: durationpb.New(6 * time.Second),
								RequestTimeout: durationpb.New(7 * time.Second)},
							"port3": {
								ConnectTimeout: durationpb.New(8 * time.Second),
								RequestTimeout: durationpb.New(9 * time.Second)},
						},
						InboundConnections: &pbmesh.InboundConnectionsConfig{
							MaxInboundConnections:     123,
							BalanceInboundConnections: pbmesh.BalanceConnections(pbproxystate.BalanceConnections_BALANCE_CONNECTIONS_EXACT),
						},
						ExposeConfig: &pbmesh.ExposeConfig{
							ExposePaths: []*pbmesh.ExposePath{
								{
									ListenerPort:  1234,
									Path:          "/health",
									LocalPathPort: 9090,
									Protocol:      pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP,
								},
								{
									ListenerPort:  1235,
									Path:          "GetHealth",
									LocalPathPort: 9091,
									Protocol:      pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP2,
								},
							},
						},
					},
				},
			},
		}

		for name, c := range cases {
			t.Run(resourcetest.AppendTenancyInfoSubtest(t.Name(), name, tenancy), func(t *testing.T) {
				proxyTmpl := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", true, c.proxyCfg).
					BuildLocalApp(c.workload, nil).
					Build()

				// sort routers because of test flakes where order was flip flopping.
				actualRouters := proxyTmpl.ProxyState.Listeners[0].Routers
				sort.Slice(actualRouters, func(i, j int) bool {
					return actualRouters[i].String() < actualRouters[j].String()
				})

				actual := protoToJSON(t, proxyTmpl)
				expected := JSONToProxyTemplate(t, golden.GetBytes(t, actual, name+"-"+tenancy.Partition+"-"+tenancy.Namespace+".golden"))

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

func TestBuildL4TrafficPermissions(t *testing.T) {
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		testTrustDomain := "test.consul"

		cases := map[string]struct {
			defaultAllow  bool
			workloadPorts map[string]*pbcatalog.WorkloadPort
			ctp           *pbauth.ComputedTrafficPermissions
			expected      map[string]*pbproxystate.TrafficPermissions
		}{
			"empty": {
				defaultAllow: true,
				workloadPorts: map[string]*pbcatalog.WorkloadPort{
					"p1": {
						Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
					},
					"p2": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					"p3": {},
					"mesh": {
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				expected: map[string]*pbproxystate.TrafficPermissions{
					"p1": {
						DefaultAllow: false,
					},
					"p2": {
						DefaultAllow: false,
					},
					"p3": {
						DefaultAllow: false,
					},
				},
			},
			"default allow everywhere": {
				defaultAllow: true,
				workloadPorts: map[string]*pbcatalog.WorkloadPort{
					"p1": {
						Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
					},
					"p2": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					"p3": {},
					"mesh": {
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				ctp: &pbauth.ComputedTrafficPermissions{
					IsDefault: true,
				},
				expected: map[string]*pbproxystate.TrafficPermissions{
					"p1": {
						DefaultAllow: true,
					},
					"p2": {
						DefaultAllow: true,
					},
					"p3": {
						DefaultAllow: true,
					},
				},
			},
			"preserves default deny": {
				defaultAllow: false,
				workloadPorts: map[string]*pbcatalog.WorkloadPort{
					"p1": {
						Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
					},
					"p2": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				ctp: &pbauth.ComputedTrafficPermissions{
					AllowPermissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "foo",
									Partition:    tenancy.Partition,
									Namespace:    tenancy.Namespace,
								},
							},
							DestinationRules: []*pbauth.DestinationRule{
								{
									PortNames: []string{"p1"},
								},
							},
						},
					},
				},
				expected: map[string]*pbproxystate.TrafficPermissions{
					"p1": {
						DefaultAllow: false,
						AllowPermissions: []*pbproxystate.Permission{
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/foo$", tenancy.Partition, tenancy.Namespace)},
									},
								},
							},
						},
					},
					"p2": {
						DefaultAllow: false,
					},
				},
			},
			"preserves default deny wildcard namespace": {
				defaultAllow: false,
				workloadPorts: map[string]*pbcatalog.WorkloadPort{
					"p1": {
						Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
					},
					"p2": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				ctp: &pbauth.ComputedTrafficPermissions{
					AllowPermissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "",
									Partition:    tenancy.Partition,
									Namespace:    "",
								},
							},
							DestinationRules: []*pbauth.DestinationRule{
								{
									PortNames: []string{"p1"},
								},
							},
						},
					},
				},
				expected: map[string]*pbproxystate.TrafficPermissions{
					"p1": {
						DefaultAllow: false,
						AllowPermissions: []*pbproxystate.Permission{
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/[^/]+$", tenancy.Partition, anyPath)},
									},
								},
							},
						},
					},
					"p2": {
						DefaultAllow: false,
					},
				},
			},
			"preserves default deny wildcard namespace exclude source": {
				defaultAllow: false,
				workloadPorts: map[string]*pbcatalog.WorkloadPort{
					"p1": {
						Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
					},
					"p2": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				ctp: &pbauth.ComputedTrafficPermissions{
					AllowPermissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "",
									Partition:    tenancy.Partition,
									Namespace:    "",
									Exclude: []*pbauth.ExcludeSource{
										{
											IdentityName: "bar",
											Namespace:    tenancy.Namespace,
											Partition:    tenancy.Partition,
										},
									},
								},
							},
							DestinationRules: []*pbauth.DestinationRule{
								{
									PortNames: []string{"p1"},
								},
							},
						},
					},
				},
				expected: map[string]*pbproxystate.TrafficPermissions{
					"p1": {
						DefaultAllow: false,
						AllowPermissions: []*pbproxystate.Permission{
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/[^/]+$", tenancy.Partition, anyPath)},

										ExcludeSpiffes: []*pbproxystate.Spiffe{
											{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/bar$", tenancy.Partition, tenancy.Namespace)},
										},
									},
								},
							},
						},
					},
					"p2": {
						DefaultAllow: false,
					},
				},
			},
			"default allow with a non-empty ctp becomes default deny on all ports": {
				defaultAllow: true,
				workloadPorts: map[string]*pbcatalog.WorkloadPort{
					"p1": {
						Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
					},
					"p2": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				ctp: &pbauth.ComputedTrafficPermissions{
					AllowPermissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "baz",
									Partition:    tenancy.Partition,
									Namespace:    tenancy.Namespace,
								},
							},
							DestinationRules: []*pbauth.DestinationRule{
								{
									PortNames: []string{"no-match"},
								},
							},
						},
					},
				},
				expected: map[string]*pbproxystate.TrafficPermissions{
					"p1": {
						DefaultAllow: false,
					},
					"p2": {
						DefaultAllow: false,
					},
				},
			},
			"kitchen sink": {
				defaultAllow: true,
				workloadPorts: map[string]*pbcatalog.WorkloadPort{
					"p1": {
						Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
					},
					"p2": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				ctp: &pbauth.ComputedTrafficPermissions{
					AllowPermissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "foo",
									Partition:    tenancy.Partition,
									Namespace:    tenancy.Namespace,
								},
								{
									IdentityName: "",
									Partition:    tenancy.Partition,
									Namespace:    tenancy.Namespace,
									Exclude: []*pbauth.ExcludeSource{
										{
											IdentityName: "bar",
											Namespace:    tenancy.Namespace,
											Partition:    tenancy.Partition,
										},
									},
								},
							},
							DestinationRules: []*pbauth.DestinationRule{
								// This should be p2.
								{
									Exclude: []*pbauth.ExcludePermissionRule{
										{
											PortNames: []string{"p1"},
										},
									},
								},
							},
						},
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "baz",
									Partition:    tenancy.Partition,
									Namespace:    tenancy.Namespace,
								},
							},
							DestinationRules: []*pbauth.DestinationRule{
								{
									PortNames: []string{"p1"},
								},
							},
						},
					},
					DenyPermissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "qux",
									Partition:    tenancy.Partition,
									Namespace:    tenancy.Namespace,
								},
							},
						},
						{
							Sources: []*pbauth.Source{
								{
									IdentityName: "",
									Namespace:    tenancy.Namespace,
									Partition:    tenancy.Partition,
									Exclude: []*pbauth.ExcludeSource{
										{
											IdentityName: "quux",
											Partition:    tenancy.Partition,
											Namespace:    tenancy.Namespace,
										},
									},
								},
							},
						},
					},
				},
				expected: map[string]*pbproxystate.TrafficPermissions{
					"p1": {
						DefaultAllow: false,
						DenyPermissions: []*pbproxystate.Permission{
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/qux$", tenancy.Partition, tenancy.Namespace)},
									},
								},
							},
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf(`^spiffe://test.consul/ap/%s/ns/%s/identity/[^/]+$`, tenancy.Partition, tenancy.Namespace)},
										ExcludeSpiffes: []*pbproxystate.Spiffe{
											{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/quux$", tenancy.Partition, tenancy.Namespace)},
										},
									},
								},
							},
						},
						AllowPermissions: []*pbproxystate.Permission{
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/baz$", tenancy.Partition, tenancy.Namespace)},
									},
								},
							},
						},
					},
					"p2": {
						DefaultAllow: false,
						DenyPermissions: []*pbproxystate.Permission{
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/qux$", tenancy.Partition, tenancy.Namespace)},
									},
								},
							},
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf(`^spiffe://test.consul/ap/%s/ns/%s/identity/[^/]+$`, tenancy.Partition, tenancy.Namespace)},
										ExcludeSpiffes: []*pbproxystate.Spiffe{
											{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/quux$", tenancy.Partition, tenancy.Namespace)},
										},
									},
								},
							},
						},
						AllowPermissions: []*pbproxystate.Permission{
							{
								Principals: []*pbproxystate.Principal{
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/foo$", tenancy.Partition, tenancy.Namespace)},
									},
									{
										Spiffe: &pbproxystate.Spiffe{Regex: fmt.Sprintf(`^spiffe://test.consul/ap/%s/ns/%s/identity/[^/]+$`, tenancy.Partition, tenancy.Namespace)},
										ExcludeSpiffes: []*pbproxystate.Spiffe{
											{Regex: fmt.Sprintf("^spiffe://test.consul/ap/%s/ns/%s/identity/bar$", tenancy.Partition, tenancy.Namespace)},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		for name, tc := range cases {
			t.Run(resourcetest.AppendTenancyInfoSubtest(t.Name(), name, tenancy), func(t *testing.T) {
				workload := &pbcatalog.Workload{
					Ports: tc.workloadPorts,
				}
				permissions := buildTrafficPermissions(tc.defaultAllow, testTrustDomain, workload, tc.ctp)
				require.Equal(t, len(tc.expected), len(permissions))
				for k, v := range tc.expected {
					prototest.AssertDeepEqual(t, v, permissions[k])
				}
			})
		}
	}, t)
}

func testProxyStateTemplateID(tenancy *pbresource.Tenancy) *pbresource.ID {
	return resourcetest.Resource(pbmesh.ProxyStateTemplateType, "test").
		WithTenancy(tenancy).
		ID()
}

func testIdentityRef(tenancy *pbresource.Tenancy) *pbresource.Reference {
	return &pbresource.Reference{
		Name:    "test-identity",
		Tenancy: tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}
}
