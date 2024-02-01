// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	"testing"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestValidateMeshGateway(t *testing.T) {
	type testcase struct {
		mgwName   string
		mgw       *pbmesh.MeshGateway
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.MeshGatewayType, tc.mgwName).
			WithData(t, tc.mgw).
			Build()

		err := resource.DecodeAndValidate(validateMeshGateway)(res)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"happy path": {
			mgwName: "mesh-gateway",
			mgw: &pbmesh.MeshGateway{
				Listeners: []*pbmesh.MeshGatewayListener{
					{
						Name: "wan",
					},
				},
			},
			expectErr: "",
		},
		"wrong name for mesh-gateway": {
			mgwName: "my-mesh-gateway",
			mgw: &pbmesh.MeshGateway{
				Listeners: []*pbmesh.MeshGatewayListener{
					{
						Name: "wan",
					},
				},
			},
			expectErr: "invalid gateway name, must be \"mesh-gateway\"",
		},
		"too many listeners on mesh-gateway": {
			mgwName: "mesh-gateway",
			mgw: &pbmesh.MeshGateway{
				Listeners: []*pbmesh.MeshGatewayListener{
					{
						Name: "obi",
					},
					{
						Name: "wan",
					},
				},
			},
			expectErr: "invalid listeners, must have exactly one listener",
		},
		"zero listeners on mesh-gateway": {
			mgwName:   "mesh-gateway",
			mgw:       &pbmesh.MeshGateway{},
			expectErr: "invalid listeners, must have exactly one listener",
		},
		"incorrect listener name on mesh-gateway": {
			mgwName: "mesh-gateway",
			mgw: &pbmesh.MeshGateway{
				Listeners: []*pbmesh.MeshGatewayListener{
					{
						Name: "kenobi",
					},
				},
			},
			expectErr: "invalid listener name, must be \"wan\"",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
