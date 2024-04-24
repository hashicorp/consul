// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidatePartitionTrafficPermissions_ParseError(t *testing.T) {
	data := &pbauth.ComputedTrafficPermissions{AllowPermissions: nil}

	res := resourcetest.Resource(pbauth.PartitionTrafficPermissionsType, "ptp").
		WithData(t, data).
		Build()

	err := ValidatePartitionTrafficPermissions(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidatePartitionTrafficPermissions_Permissions(t *testing.T) {
	for n, tc := range permissionsTestCases() {
		t.Run(n, func(t *testing.T) {
			tp := &pbauth.PartitionTrafficPermissions{
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{tc.p},
			}

			res := resourcetest.Resource(pbauth.PartitionTrafficPermissionsType, "ptp").
				WithTenancy(resource.DefaultPartitionedTenancy()).
				WithData(t, tp).
				Build()

			err := MutatePartitionTrafficPermissions(res)
			require.NoError(t, err)

			err = ValidatePartitionTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestMutatePartitionTrafficPermissions(t *testing.T) {
	run := func(t *testing.T, tc mutationTestCase) {
		tenancy := tc.tenancy
		if tenancy == nil {
			tenancy = resource.DefaultPartitionedTenancy()
		}
		res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "ntp").
			WithTenancy(tenancy).
			WithData(t, &pbauth.PartitionTrafficPermissions{
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: tc.permissions,
			}).
			Build()

		err := MutatePartitionTrafficPermissions(res)

		got := resourcetest.MustDecode[*pbauth.PartitionTrafficPermissions](t, res)

		if tc.expectErr == "" {
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, tc.expect, got.Data.Permissions)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	for name, tc := range mutationTestCases() {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPartitionTrafficPermissionsACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	ptpData := &pbauth.PartitionTrafficPermissions{
		Action: pbauth.Action_ACTION_ALLOW,
	}

	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    ptpData,
			Typ:     pbauth.PartitionTrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"operator read": {
			Rules:   `operator = "read"`,
			Data:    ptpData,
			Typ:     pbauth.PartitionTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"operator write": {
			Rules:   `operator = "write"`,
			Data:    ptpData,
			Typ:     pbauth.PartitionTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"mesh read": {
			Rules:   `mesh = "read"`,
			Data:    ptpData,
			Typ:     pbauth.PartitionTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"mesh write": {
			Rules:   `mesh = "write"`,
			Data:    ptpData,
			Typ:     pbauth.PartitionTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}
