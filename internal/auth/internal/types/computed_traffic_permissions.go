// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ComputedTrafficPermissionsKind = "ComputedTrafficPermissions"
)

var (
	ComputedTrafficPermissionsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ComputedTrafficPermissionsKind,
	}

	ComputedTrafficPermissionsType = ComputedTrafficPermissionsV1Alpha1Type
)

func RegisterComputedTrafficPermission(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ComputedTrafficPermissionsV1Alpha1Type,
		Proto:    &pbauth.ComputedTrafficPermissions{},
		Scope:    resource.ScopeNamespace,
		Validate: nil,
	})
}
