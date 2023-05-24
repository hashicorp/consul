// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource_test

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestRegister(t *testing.T) {
	r := resource.NewRegistry()

	// register success
	reg := resource.Registration{Type: demo.TypeV2Artist}
	r.Register(reg)
	actual, ok := r.Resolve(demo.TypeV2Artist)
	require.True(t, ok)
	require.True(t, proto.Equal(demo.TypeV2Artist, actual.Type))

	// register existing should panic
	require.PanicsWithValue(t, "resource type demo.v2.artist already registered", func() {
		r.Register(reg)
	})

	// type missing required fields should panic
	testcases := map[string]*pbresource.Type{
		"empty group": {
			Group:        "",
			GroupVersion: "v2",
			Kind:         "artist",
		},
		"empty group version": {
			Group:        "",
			GroupVersion: "v2",
			Kind:         "artist",
		},
		"empty kind": {
			Group:        "demo",
			GroupVersion: "v2",
			Kind:         "",
		},
	}

	for desc, typ := range testcases {
		t.Run(desc, func(t *testing.T) {
			require.PanicsWithValue(t, "type field(s) cannot be empty", func() {
				r.Register(resource.Registration{Type: typ})
			})
		})
	}
}

func TestRegister_Defaults(t *testing.T) {
	r := resource.NewRegistry()
	r.Register(resource.Registration{Type: demo.TypeV2Artist})
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	reg, ok := r.Resolve(demo.TypeV2Artist)
	require.True(t, ok)

	// verify default read hook requires operator:read
	require.NoError(t, reg.ACLs.Read(testutils.ACLOperatorRead(t), artist.Id))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.Read(testutils.ACLNoPermissions(t), artist.Id)))

	// verify default write hook requires operator:write
	require.NoError(t, reg.ACLs.Write(testutils.ACLOperatorWrite(t), artist.Id))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.Write(testutils.ACLNoPermissions(t), artist.Id)))

	// verify default list hook requires operator:read
	require.NoError(t, reg.ACLs.List(testutils.ACLOperatorRead(t), artist.Id.Tenancy))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.List(testutils.ACLNoPermissions(t), artist.Id.Tenancy)))

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

	serviceType := &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v1",
		Kind:         "service",
	}

	// not found
	_, ok := r.Resolve(serviceType)
	assert.False(t, ok)

	// found
	r.Register(resource.Registration{Type: serviceType})
	registration, ok := r.Resolve(serviceType)
	assert.True(t, ok)
	assert.Equal(t, registration.Type, serviceType)
}
