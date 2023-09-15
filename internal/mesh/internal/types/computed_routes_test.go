// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidateComputedRoutes(t *testing.T) {
	type testcase struct {
		routes    *pbmesh.ComputedRoutes
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(ComputedRoutesType, "api").
			WithData(t, tc.routes).
			Build()

		err := ValidateComputedRoutes(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.ComputedRoutes](t, res)
		prototest.AssertDeepEqual(t, tc.routes, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"empty": {
			routes:    &pbmesh.ComputedRoutes{},
			expectErr: `invalid "ported_configs" field: cannot be empty`,
		},
		"empty config": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: nil,
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid "config" field: cannot be empty`,
		},
		"target/missing mesh port": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {
								MeshPort: "",
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "mesh_port" field: cannot be empty`,
		},
		"target/should not have service endpoints id": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {
								MeshPort:           "mesh",
								ServiceEndpointsId: &pbresource.ID{},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "service_endpoints_id" field: field should be empty`,
		},
		"target/should not have service endpoints": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {
								MeshPort:         "mesh",
								ServiceEndpoints: &pbcatalog.ServiceEndpoints{},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "service_endpoints" field: field should be empty`,
		},
		"target/should not have identity refs": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {
								MeshPort: "mesh",
								IdentityRefs: []*pbresource.Reference{
									{},
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "identity_refs" field: field should be empty`,
		},
		"valid": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {
								MeshPort: "mesh",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
