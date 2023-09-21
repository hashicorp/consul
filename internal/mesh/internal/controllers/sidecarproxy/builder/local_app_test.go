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

func TestBuildL4TrafficPermissions(t *testing.T) {
	testTrustDomain := "test.consul"

	cases := map[string]struct {
		workloadPorts map[string]*pbcatalog.WorkloadPort
		ctp           *pbauth.ComputedTrafficPermissions
		expected      map[string]*pbproxystate.TrafficPermissions
	}{
		"empty": {
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
				"p1": {},
				"p2": {},
				"p3": {},
			},
		},
		"kitchen sink": {
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
			permissions := buildTrafficPermissions(testTrustDomain, workload, tc.ctp)
			require.Equal(t, len(tc.expected), len(permissions))
			for k, v := range tc.expected {
				prototest.AssertDeepEqual(t, v, permissions[k])
			}
		})
	}
}
