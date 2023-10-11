// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
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
	cases := map[string]struct {
		workload     *pbcatalog.Workload
		ctp          *pbauth.ComputedTrafficPermissions
		defaultAllow bool
	}{
		"source/l4-single-workload-address-without-ports": {
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
		},
		"source/l4-multiple-workload-addresses-without-ports": {
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
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
		"source/l4-multiple-workload-addresses-with-specific-ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "127.0.0.1",
						Ports: []string{"port1"},
					},
					{
						Host:  "10.0.0.2",
						Ports: []string{"port2"},
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
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
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", c.defaultAllow, nil).
				BuildLocalApp(c.workload, c.ctp).
				Build()
			actual := protoToJSON(t, proxyTmpl)
			expected := golden.Get(t, actual, name+".golden")

			require.JSONEq(t, expected, actual)
		})
	}
}

func TestBuildLocalApp_WithProxyConfiguration(t *testing.T) {
	cases := map[string]struct {
		workload *pbcatalog.Workload
		proxyCfg *pbmesh.ProxyConfiguration
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
			proxyCfg: &pbmesh.ProxyConfiguration{
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
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), "foo.consul", "dc1", true, c.proxyCfg).
				BuildLocalApp(c.workload, nil).
				Build()
			actual := protoToJSON(t, proxyTmpl)
			expected := golden.Get(t, actual, name+".golden")

			require.JSONEq(t, expected, actual)
		})
	}
}

func TestBuildL4TrafficPermissions(t *testing.T) {
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
								Partition:    "default",
								Namespace:    "default",
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
									Spiffe: &pbproxystate.Spiffe{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/foo$"},
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
								Partition:    "default",
								Namespace:    "default",
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
								Partition:    "default",
								Namespace:    "default",
							},
							{
								IdentityName: "",
								Partition:    "default",
								Namespace:    "default",
								Exclude: []*pbauth.ExcludeSource{
									{
										IdentityName: "bar",
										Namespace:    "default",
										Partition:    "default",
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
								Partition:    "default",
								Namespace:    "default",
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
								Partition:    "default",
								Namespace:    "default",
							},
						},
					},
					{
						Sources: []*pbauth.Source{
							{
								IdentityName: "",
								Namespace:    "default",
								Partition:    "default",
								Exclude: []*pbauth.ExcludeSource{
									{
										IdentityName: "quux",
										Partition:    "default",
										Namespace:    "default",
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
									Spiffe: &pbproxystate.Spiffe{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/qux$"},
								},
							},
						},
						{
							Principals: []*pbproxystate.Principal{
								{
									Spiffe: &pbproxystate.Spiffe{Regex: `^spiffe://test.consul/ap/default/ns/default/identity/[^/]+$`},
									ExcludeSpiffes: []*pbproxystate.Spiffe{
										{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/quux$"},
									},
								},
							},
						},
					},
					AllowPermissions: []*pbproxystate.Permission{
						{
							Principals: []*pbproxystate.Principal{
								{
									Spiffe: &pbproxystate.Spiffe{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/baz$"},
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
									Spiffe: &pbproxystate.Spiffe{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/qux$"},
								},
							},
						},
						{
							Principals: []*pbproxystate.Principal{
								{
									Spiffe: &pbproxystate.Spiffe{Regex: `^spiffe://test.consul/ap/default/ns/default/identity/[^/]+$`},
									ExcludeSpiffes: []*pbproxystate.Spiffe{
										{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/quux$"},
									},
								},
							},
						},
					},
					AllowPermissions: []*pbproxystate.Permission{
						{
							Principals: []*pbproxystate.Principal{
								{
									Spiffe: &pbproxystate.Spiffe{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/foo$"},
								},
								{
									Spiffe: &pbproxystate.Spiffe{Regex: `^spiffe://test.consul/ap/default/ns/default/identity/[^/]+$`},
									ExcludeSpiffes: []*pbproxystate.Spiffe{
										{Regex: "^spiffe://test.consul/ap/default/ns/default/identity/bar$"},
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
		t.Run(name, func(t *testing.T) {
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
}

func testProxyStateTemplateID() *pbresource.ID {
	return resourcetest.Resource(pbmesh.ProxyStateTemplateType, "test").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		ID()
}

func testIdentityRef() *pbresource.Reference {
	return &pbresource.Reference{
		Name: "test-identity",
		Tenancy: &pbresource.Tenancy{
			Namespace: "default",
			Partition: "default",
			PeerName:  "local",
		},
	}
}
