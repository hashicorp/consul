// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidateComputedTrafficPermissions_Permissions(t *testing.T) {
	for n, tc := range permissionsTestCases() {
		t.Run(n, func(t *testing.T) {

			for _, s := range tc.p.Sources {
				normalizedTenancyForSource(s, resource.DefaultNamespacedTenancy())
			}

			allowCTP := &pbauth.ComputedTrafficPermissions{
				AllowPermissions: []*pbauth.Permission{tc.p},
			}

			denyCTP := &pbauth.ComputedTrafficPermissions{
				DenyPermissions: []*pbauth.Permission{tc.p},
			}

			for _, ctp := range []*pbauth.ComputedTrafficPermissions{allowCTP, denyCTP} {
				res := resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, "tp").
					WithData(t, ctp).
					Build()

				err := ValidateComputedTrafficPermissions(res)
				if tc.expectErr == "" {
					require.NoError(t, err)
				} else {
					testutil.RequireErrorContains(t, err, tc.expectErr)
				}
			}
		})
	}
}
