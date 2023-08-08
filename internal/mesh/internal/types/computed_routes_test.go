// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
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
		got := resourcetest.MustDecode[pbmesh.ComputedRoutes, *pbmesh.ComputedRoutes](t, res)
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
		"empty targets": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.InterpretedTCPRoute{},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within ported_configs: invalid "targets" field: cannot be empty`,
		},
		"valid": {
			routes: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.InterpretedTCPRoute{},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							"foo": {},
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
