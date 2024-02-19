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

func TestValidateNamespaceTrafficPermissions_ParseError(t *testing.T) {
	data := &pbauth.ComputedTrafficPermissions{AllowPermissions: nil}

	res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "ntp").
		WithData(t, data).
		Build()

	err := ValidateNamespaceTrafficPermissions(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateNamespaceTrafficPermissions_Permissions(t *testing.T) {
	for n, tc := range permissionsTestCases() {
		t.Run(n, func(t *testing.T) {
			tp := &pbauth.NamespaceTrafficPermissions{
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{tc.p},
			}

			res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "tp").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, tp).
				Build()

			err := MutateNamespaceTrafficPermissions(res)
			require.NoError(t, err)

			err = ValidateNamespaceTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestMutateNamespaceTrafficPermissions(t *testing.T) {
	run := func(t *testing.T, tc mutationTestCase) {
		tenancy := tc.tenancy
		if tenancy == nil {
			tenancy = resource.DefaultNamespacedTenancy()
		}
		res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "ntp").
			WithTenancy(tenancy).
			WithData(t, &pbauth.NamespaceTrafficPermissions{
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: tc.permissions,
			}).
			Build()

		err := MutateNamespaceTrafficPermissions(res)

		got := resourcetest.MustDecode[*pbauth.NamespaceTrafficPermissions](t, res)

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

func TestNamespaceTrafficPermissionsACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	ntpData := &pbauth.NamespaceTrafficPermissions{
		Action: pbauth.Action_ACTION_ALLOW,
	}

	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    ntpData,
			Typ:     pbauth.NamespaceTrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"operator read": {
			Rules:   `operator = "read"`,
			Data:    ntpData,
			Typ:     pbauth.NamespaceTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"operator write": {
			Rules:   `operator = "write"`,
			Data:    ntpData,
			Typ:     pbauth.NamespaceTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"mesh read": {
			Rules:   `mesh = "read"`,
			Data:    ntpData,
			Typ:     pbauth.NamespaceTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"mesh write": {
			Rules:   `mesh = "write"`,
			Data:    ntpData,
			Typ:     pbauth.NamespaceTrafficPermissionsType,
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
