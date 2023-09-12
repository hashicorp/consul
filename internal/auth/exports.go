// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// API Group Information

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion

	// Resource Kind Names.

	WorkloadIdentity   = types.WorkloadIdentityKind
	TrafficPermissions = types.TrafficPermissionsKind

	// Resource Types for the v1alpha1 version.

	WorkloadIdentityV1Alpha1Type           = types.WorkloadIdentityV1Alpha1Type
	TrafficPermissionsV1Alpha1Type         = types.TrafficPermissionsV1Alpha1Type
	ComputedTrafficPermissionsV1Alpha1Type = types.ComputedTrafficPermissionsV1Alpha1Type

	// Resource Types for the latest version.

	WorkloadIdentityType           = types.WorkloadIdentityType
	TrafficPermissionsType         = types.TrafficPermissionsType
	ComputedTrafficPermissionsType = types.ComputedTrafficPermissionsType

	// Controller Statuses
	// TODO
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
