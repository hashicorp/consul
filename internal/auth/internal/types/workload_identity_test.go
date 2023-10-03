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
)

func TestWorkloadIdentityACLs(t *testing.T) {
	const (
		DENY    = "deny"
		ALLOW   = "allow"
		DEFAULT = "default"
	)

	registry := resource.NewRegistry()
	Register(registry)

	reg, ok := registry.Resolve(pbauth.WorkloadIdentityType)
	require.True(t, ok)

	type testcase struct {
		rules   string
		check   func(t *testing.T, authz acl.Authorizer, res *pbresource.Resource)
		readOK  string
		writeOK string
		listOK  string
	}

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

	run := func(t *testing.T, tc testcase) {
		wid := &pbauth.WorkloadIdentity{}
		res := resourcetest.Resource(pbauth.WorkloadIdentityType, "wi1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, wid).
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
		t.Run("errors", func(t *testing.T) {
			require.ErrorIs(t, reg.ACLs.Read(authz, &acl.AuthorizerContext{}, nil, nil), resource.ErrNeedData)
			require.ErrorIs(t, reg.ACLs.Write(authz, &acl.AuthorizerContext{}, nil), resource.ErrNeedData)
		})
	}

	cases := map[string]testcase{
		"no rules": {
			rules:   ``,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity wi1 read, no intentions": {
			rules:   `identity "wi1" { policy = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity wi1 read, deny intentions has no effect": {
			rules:   `identity "wi1" { policy = "read", intentions = "deny" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity wi1 read, intentions read has no effect": {
			rules:   `identity "wi1" { policy = "read", intentions = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity wi1 write, write intentions has no effect": {
			rules:   `identity "wi1" { policy = "read", intentions = "write" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity wi1 write, deny intentions has no effect": {
			rules:   `identity "wi1" { policy = "write", intentions = "deny" }`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
		"workload identity wi1 write, intentions read has no effect": {
			rules:   `identity "wi1" { policy = "write", intentions = "read" }`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
		"workload identity wi1 write, intentions write": {
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
