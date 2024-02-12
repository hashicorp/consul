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

func TestComputedTrafficPermissionsACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	ctpData := &pbauth.ComputedTrafficPermissions{
		AllowPermissions: []*pbauth.Permission{},
		DenyPermissions:  []*pbauth.Permission{},
		IsDefault:        true,
	}

	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 read, no intentions": {
			Rules:   `identity "test" { policy = "read" }`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 read, deny intentions": {
			Rules:   `identity "test" { policy = "read", intentions = "deny" }`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 read, intentions read": {
			Rules:   `identity "test" { policy = "read", intentions = "read" }`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 write, write intentions": {
			Rules:   `identity "test" { policy = "read", intentions = "write" }`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY, // users should not write computed resources
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 write, deny intentions": {
			Rules:   `identity "test" { policy = "write", intentions = "deny" }`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 write, intentions read": {
			Rules:   `identity "test" { policy = "write", intentions = "read" }`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 write, intentions write": {
			Rules:   `identity "test" { policy = "write", intentions = "write" }`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY, // users should not write computed resources
			ListOK:  resourcetest.DEFAULT,
		},
		"operator read": {
			Rules:   `operator = "read"`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY, // users should not write computed resources
			ListOK:  resourcetest.DEFAULT,
		},
		"operator write": {
			Rules:   `operator = "write"`,
			Data:    ctpData,
			Typ:     pbauth.ComputedTrafficPermissionsType,
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
