// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
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
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	type testcase struct {
		rules   string
		check   func(t *testing.T, authz acl.Authorizer, res *pbresource.Resource)
		readOK  string
		writeOK string
		listOK  string
	}

	const (
		DENY    = "deny"
		ALLOW   = "allow"
		DEFAULT = "default"
	)

	checkF := func(t *testing.T, expect string, got error) {
		switch expect {
		case ALLOW:
			if acl.IsErrPermissionDenied(got) {
				t.Fatal("should be allowed")
			}
		case DENY:
			if !acl.IsErrPermissionDenied(got) {
				t.Fatal("should be denied")
			}
		case DEFAULT:
			require.Nil(t, got, "expected fallthrough decision")
		default:
			t.Fatalf("unexpected expectation: %q", expect)
		}
	}

	reg, ok := registry.Resolve(pbauth.ComputedTrafficPermissionsType)
	require.True(t, ok)

	run := func(t *testing.T, tc testcase) {
		ctpData := &pbauth.ComputedTrafficPermissions{}
		res := resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, "wi1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, ctpData).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res)

		config := acl.Config{
			WildcardName: structs.WildcardSpecifier,
		}
		authz, err := acl.NewAuthorizerFromRules(tc.rules, &config, nil)
		require.NoError(t, err)
		authz = acl.NewChainedAuthorizer([]acl.Authorizer{authz, acl.DenyAll()})

		t.Run("read", func(t *testing.T) {
			err := reg.ACLs.Read(authz, &acl.AuthorizerContext{}, res.Id, res)
			checkF(t, tc.readOK, err)
		})
		t.Run("write", func(t *testing.T) {
			err := reg.ACLs.Write(authz, &acl.AuthorizerContext{}, res)
			checkF(t, tc.writeOK, err)
		})
		t.Run("list", func(t *testing.T) {
			err := reg.ACLs.List(authz, &acl.AuthorizerContext{})
			checkF(t, tc.listOK, err)
		})
	}

	cases := map[string]testcase{
		"no rules": {
			rules:   ``,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 read, no intentions": {
			rules:   `identity "wi1" { policy = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 read, deny intentions": {
			rules:   `identity "wi1" { policy = "read", intentions = "deny" }`,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 read, intentions read": {
			rules:   `identity "wi1" { policy = "read", intentions = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, write intentions": {
			rules:   `identity "wi1" { policy = "read", intentions = "write" }`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, deny intentions": {
			rules:   `identity "wi1" { policy = "write", intentions = "deny" }`,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, intentions read": {
			rules:   `identity "wi1" { policy = "write", intentions = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, intentions write": {
			rules:   `identity "wi1" { policy = "write", intentions = "write" }`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
