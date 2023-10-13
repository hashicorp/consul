// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	DENY    = "deny"
	ALLOW   = "allow"
	DEFAULT = "default"
)

var checkF = func(t *testing.T, expect string, got error) {
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

type ACLTestCase struct {
	Rules   string
	Data    protoreflect.ProtoMessage
	Owner   *pbresource.ID
	Typ     *pbresource.Type
	ReadOK  string
	WriteOK string
	ListOK  string
}

func RunACLTestCase(t *testing.T, tc ACLTestCase, registry resource.Registry) {
	reg, ok := registry.Resolve(tc.Typ)
	require.True(t, ok)

	resolvedType, ok := registry.Resolve(tc.Typ)
	require.True(t, ok)

	res := Resource(tc.Typ, "test").
		WithTenancy(DefaultTenancyForType(t, resolvedType)).
		WithOwner(tc.Owner).
		WithData(t, tc.Data).
		Build()

	ValidateAndNormalize(t, registry, res)

	config := acl.Config{
		WildcardName: structs.WildcardSpecifier,
	}
	authz, err := acl.NewAuthorizerFromRules(tc.Rules, &config, nil)
	require.NoError(t, err)
	authz = acl.NewChainedAuthorizer([]acl.Authorizer{authz, acl.DenyAll()})

	t.Run("read", func(t *testing.T) {
		err := reg.ACLs.Read(authz, &acl.AuthorizerContext{}, res.Id, res)
		checkF(t, tc.ReadOK, err)
	})
	t.Run("write", func(t *testing.T) {
		err := reg.ACLs.Write(authz, &acl.AuthorizerContext{}, res)
		checkF(t, tc.WriteOK, err)
	})
	t.Run("list", func(t *testing.T) {
		err := reg.ACLs.List(authz, &acl.AuthorizerContext{})
		checkF(t, tc.ListOK, err)
	})
}
