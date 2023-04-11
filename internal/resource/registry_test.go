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
)

func TestRegister(t *testing.T) {
	r := resource.NewRegistry()

	serviceType := &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v1",
		Kind:         "service",
	}

	// register success
	serviceRegistration := resource.Registration{Type: serviceType}
	r.Register(serviceRegistration)

	// register existing should panic
	assertRegisterPanics(t, r.Register, serviceRegistration, "resource type mesh.v1.service already registered")

	// register empty Group should panic
	assertRegisterPanics(t, r.Register, resource.Registration{
		Type: &pbresource.Type{
			Group:        "",
			GroupVersion: "v1",
			Kind:         "service",
		},
	}, "type field(s) cannot be empty")

	// register empty GroupVersion should panic
	assertRegisterPanics(t, r.Register, resource.Registration{
		Type: &pbresource.Type{
			Group:        "mesh",
			GroupVersion: "",
			Kind:         "service",
		},
	}, "type field(s) cannot be empty")

	// register empty Kind should panic
	assertRegisterPanics(t, r.Register, resource.Registration{
		Type: &pbresource.Type{
			Group:        "mesh",
			GroupVersion: "v1",
			Kind:         "",
		},
	}, "type field(s) cannot be empty")
}

func TestRegister_Defaults(t *testing.T) {
	r := resource.NewRegistry()
	r.Register(resource.Registration{
		Type: demo.TypeV2Artist,
		// intentionally don't provide ACLs so defaults kick in
	})
	artist, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	reg, ok := r.Resolve(demo.TypeV2Artist)
	require.True(t, ok)

	// verify default read hook requires operator:read
	require.NoError(t, reg.ACLs.Read(testutils.ACLOperatorRead(t), artist.Id))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.Read(testutils.ACLNoPermissions(t), artist.Id)))

	// verify default write hook requires operator:write
	require.NoError(t, reg.ACLs.Write(testutils.ACLOperatorWrite(t), artist))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.Write(testutils.ACLNoPermissions(t), artist)))

	// verify default list hook requires operator:read
	require.NoError(t, reg.ACLs.List(testutils.ACLOperatorRead(t), artist.Id.Tenancy))
	require.True(t, acl.IsErrPermissionDenied(reg.ACLs.List(testutils.ACLNoPermissions(t), artist.Id.Tenancy)))

	// verify default validate is a no-op
	require.NoError(t, reg.Validate(nil))
}

func assertRegisterPanics(t *testing.T, registerFn func(reg resource.Registration), registration resource.Registration, panicString string) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic, but none occurred")
		} else {
			errstr, ok := r.(string)
			if !ok {
				t.Errorf("unexpected error type returned from panic")
			} else if errstr != panicString {
				t.Errorf("expected %s error message but got: %s", panicString, errstr)
			}
		}
	}()

	registerFn(registration)
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
