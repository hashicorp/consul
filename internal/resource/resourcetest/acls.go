// Copyright IBM Corp. 2014, 2025
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
	Rules string

	// AuthCtx is optional. If not provided an empty one will be used.
	AuthCtx *acl.AuthorizerContext

	// One of either Res or Data/Owner/Typ should be set.
	Res   *pbresource.Resource
	Data  protoreflect.ProtoMessage
	Owner *pbresource.ID
	Typ   *pbresource.Type

	ReadOK  string
	WriteOK string
	ListOK  string

	ReadHookRequiresResource bool
}

func RunACLTestCase(t *testing.T, tc ACLTestCase, registry resource.Registry) {
	var (
		typ *pbresource.Type
		res *pbresource.Resource
	)
	if tc.Res != nil {
		require.Nil(t, tc.Data)
		require.Nil(t, tc.Owner)
		require.Nil(t, tc.Typ)
		typ = tc.Res.Id.GetType()
		res = tc.Res
	} else {
		require.NotNil(t, tc.Data)
		require.NotNil(t, tc.Typ)
		typ = tc.Typ

		resolvedType, ok := registry.Resolve(typ)
		require.True(t, ok)

		res = Resource(tc.Typ, "test").
			WithTenancy(DefaultTenancyForType(t, resolvedType)).
			WithOwner(tc.Owner).
			WithData(t, tc.Data).
			Build()
	}

	reg, ok := registry.Resolve(typ)
	require.True(t, ok)

	ValidateAndNormalize(t, registry, res)

	config := acl.Config{
		WildcardName: structs.WildcardSpecifier,
	}
	authz, err := acl.NewAuthorizerFromRules(tc.Rules, &config, nil)
	require.NoError(t, err)
	authz = acl.NewChainedAuthorizer([]acl.Authorizer{authz, acl.DenyAll()})

	if tc.AuthCtx == nil {
		tc.AuthCtx = &acl.AuthorizerContext{}
	}

	if tc.ReadHookRequiresResource {
		err = reg.ACLs.Read(authz, tc.AuthCtx, res.Id, nil)
		require.ErrorIs(t, err, resource.ErrNeedResource, "read hook should require the data payload")
	}

	t.Run("read", func(t *testing.T) {
		err := reg.ACLs.Read(authz, tc.AuthCtx, res.Id, res)
		checkF(t, tc.ReadOK, err)
	})
	t.Run("write", func(t *testing.T) {
		err := reg.ACLs.Write(authz, tc.AuthCtx, res)
		checkF(t, tc.WriteOK, err)
	})
	t.Run("list", func(t *testing.T) {
		err := reg.ACLs.List(authz, tc.AuthCtx)
		checkF(t, tc.ListOK, err)
	})
}
