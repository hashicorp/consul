// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"

	pbdemov1 "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	demov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestRegister(t *testing.T) {
	r := resource.NewRegistry()

	// register success
	reg := resource.Registration{
		Type:  demo.TypeV2Artist,
		Proto: &demov2.Artist{},
		Scope: resource.ScopeNamespace,
	}
	r.Register(reg)
	actual, ok := r.Resolve(demo.TypeV2Artist)
	require.True(t, ok)
	require.True(t, proto.Equal(demo.TypeV2Artist, actual.Type))

	// register existing should panic
	require.PanicsWithValue(t, "resource type demo.v2.Artist already registered", func() {
		r.Register(reg)
	})

	// register success when scope undefined and type exempt from scope
	// skip: can't test this because tombstone type is registered as part of NewRegistry()
}

func TestRegister_Defaults(t *testing.T) {
	r := resource.NewRegistry()
	r.Register(resource.Registration{
		Type:  demo.TypeV2Artist,
		Proto: &demov2.Artist{},
		Scope: resource.ScopeNamespace,
	})
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	reg, ok := r.Resolve(demo.TypeV2Artist)
	require.True(t, ok)

	// verify default read hook requires operator:read
	require.NoError(t, reg.ACLs.Read(testutils.ACLOperatorRead(t), nil, artist.Id))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.Read(testutils.ACLNoPermissions(t), nil, artist.Id)))

	// verify default write hook requires operator:write
	require.NoError(t, reg.ACLs.Write(testutils.ACLOperatorWrite(t), nil, artist))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.Write(testutils.ACLNoPermissions(t), nil, artist)))

	// verify default list hook requires operator:read
	require.NoError(t, reg.ACLs.List(testutils.ACLOperatorRead(t), nil))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.List(testutils.ACLNoPermissions(t), nil)))

	// verify default validate is a no-op
	require.NoError(t, reg.Validate(nil))

	// verify default mutate is a no-op
	require.NoError(t, reg.Mutate(nil))
}

func TestNewRegistry(t *testing.T) {
	r := resource.NewRegistry()

	// verify tombstone type registered implicitly
	_, ok := r.Resolve(resource.TypeV1Tombstone)
	require.True(t, ok)
}

func TestResolve(t *testing.T) {
	r := resource.NewRegistry()

	// not found
	_, ok := r.Resolve(demo.TypeV1Album)
	assert.False(t, ok)

	// found
	r.Register(resource.Registration{
		Type:  demo.TypeV1Album,
		Proto: &pbdemov1.Album{},
		Scope: resource.ScopeNamespace,
	})
	registration, ok := r.Resolve(demo.TypeV1Album)
	assert.True(t, ok)
	assert.Equal(t, registration.Type, demo.TypeV1Album)
}

func TestRegister_TypeValidation(t *testing.T) {
	registry := resource.NewRegistry()

	testCases := map[string]struct {
		fn    func(*pbresource.Type)
		valid bool
	}{
		"Valid": {valid: true},
		"Group empty": {
			fn:    func(t *pbresource.Type) { t.Group = "" },
			valid: false,
		},
		"Group PascalCase": {
			fn:    func(t *pbresource.Type) { t.Group = "Foo" },
			valid: false,
		},
		"Group kebab-case": {
			fn:    func(t *pbresource.Type) { t.Group = "foo-bar" },
			valid: false,
		},
		"Group snake_case": {
			fn:    func(t *pbresource.Type) { t.Group = "foo_bar" },
			valid: true,
		},
		"GroupVersion empty": {
			fn:    func(t *pbresource.Type) { t.GroupVersion = "" },
			valid: false,
		},
		"GroupVersion snake_case": {
			fn:    func(t *pbresource.Type) { t.GroupVersion = "v_1" },
			valid: false,
		},
		"GroupVersion kebab-case": {
			fn:    func(t *pbresource.Type) { t.GroupVersion = "v-1" },
			valid: false,
		},
		"GroupVersion no leading v": {
			fn:    func(t *pbresource.Type) { t.GroupVersion = "1" },
			valid: false,
		},
		"GroupVersion no trailing number": {
			fn:    func(t *pbresource.Type) { t.GroupVersion = "OnePointOh" },
			valid: false,
		},
		"Kind PascalCase with numbers": {
			fn:    func(t *pbresource.Type) { t.Kind = "Number1" },
			valid: true,
		},
		"Kind camelCase": {
			fn:    func(t *pbresource.Type) { t.Kind = "barBaz" },
			valid: false,
		},
		"Kind snake_case": {
			fn:    func(t *pbresource.Type) { t.Kind = "bar_baz" },
			valid: false,
		},
		"Kind empty": {
			fn:    func(t *pbresource.Type) { t.Kind = "" },
			valid: false,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			reg := func() {
				typ := &pbresource.Type{
					Group:        "foo",
					GroupVersion: "v1",
					Kind:         "Bar",
				}
				if tc.fn != nil {
					tc.fn(typ)
				}
				registry.Register(resource.Registration{
					Type: typ,
					// Just pass anything since proto is a required field.
					Proto: &pbdemov1.Artist{},
					// Scope is also required
					Scope: resource.ScopeNamespace,
				})
			}

			if tc.valid {
				require.NotPanics(t, reg)
			} else {
				require.Panics(t, reg)
			}
		})
	}
}
