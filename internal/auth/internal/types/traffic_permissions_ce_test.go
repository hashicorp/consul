// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import (
	"testing"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestValidateTrafficPermissionsActionCE(t *testing.T) {
	cases := map[string]struct {
		tp        *pbauth.TrafficPermissions
		expectErr string
	}{
		"ok-minimal": {
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{IdentityName: "wi-1"},
				Action:      pbauth.Action_ACTION_ALLOW,
			},
		},
		"unspecified-action": {
			// Any type other than the TrafficPermissions type would work
			// to cause the error we are expecting
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "wi1",
				},
				Action:      pbauth.Action_ACTION_UNSPECIFIED,
				Permissions: nil,
			},
			expectErr: `invalid "data.action" field: action must be allow`,
		},
		"deny-action": {
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{IdentityName: "wi-1"},
				Action:      pbauth.Action_ACTION_DENY,
			},
			expectErr: `invalid "data.action" field: action must be allow`,
		},
		"invalid-action": {
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "wi1",
				},
				Action:      pbauth.Action(50),
				Permissions: nil,
			},
			expectErr: `invalid "data.action" field: action must be allow`,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			res := resourcetest.Resource(pbauth.TrafficPermissionsType, "tp").
				WithData(t, tc.tp).
				Build()

			err := ValidateTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}
