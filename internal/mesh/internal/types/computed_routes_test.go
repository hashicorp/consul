// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
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
		res := resourcetest.Resource(pbmesh.ComputedRoutesType, "api").
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
								Type:     pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort: "",
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "mesh_port" field: cannot be empty`,
		},
		"target/missing type": {
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
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "type" field: missing required field`,
		},
		"target/bad type": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {
								Type:     99,
								MeshPort: "mesh",
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "type" field: not a supported enum value: 99`,
		},
		"target/indirect cannot have failover": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {
								Type:           pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_INDIRECT,
								MeshPort:       "mesh",
								FailoverConfig: &pbmesh.ComputedFailoverConfig{},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid value of key "foo" within targets: invalid "failover_config" field: failover_config not supported for type = INDIRECT`,
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
								Type:               pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
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
								Type:             pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
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
								Type:     pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
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
								Type:     pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort: "mesh",
								DestinationConfig: &pbmesh.DestinationConfig{
									ConnectTimeout: durationpb.New(5 * time.Second),
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
			run(t, tc)
		})
	}
}
