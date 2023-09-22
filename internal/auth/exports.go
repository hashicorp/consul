// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// API Group Information

	APIGroup       = types.GroupName
	VersionV2Beta1 = types.VersionV2Beta1
	CurrentVersion = types.CurrentVersion

	// Resource Kind Names.

	WorkloadIdentity           = types.WorkloadIdentityKind
	TrafficPermissions         = types.TrafficPermissionsKind
	ComputedTrafficPermissions = types.ComputedTrafficPermissionsKind

	// Resource Types for the v2beta1 version.

	WorkloadIdentityV2Beta1Type           = types.WorkloadIdentityV2Beta1Type
	TrafficPermissionsV2Beta1Type         = types.TrafficPermissionsV2Beta1Type
	ComputedTrafficPermissionsV2Beta1Type = types.ComputedTrafficPermissionsV2Beta1Type

	// Resource Types for the latest version.

	WorkloadIdentityType           = types.WorkloadIdentityType
	TrafficPermissionsType         = types.TrafficPermissionsType
	ComputedTrafficPermissionsType = types.ComputedTrafficPermissionsType
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
