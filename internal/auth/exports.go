// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers"
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions"
	"github.com/hashicorp/consul/internal/auth/internal/mappers/trafficpermissionsmapper"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
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

	// Controller statuses

	StatusKey                           = trafficpermissions.StatusKey
	TrafficPermissionsConditionComputed = trafficpermissions.ConditionComputed
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

type ControllerDependencies = controllers.Dependencies

func DefaultControllerDependencies() ControllerDependencies {
	return ControllerDependencies{
		WorkloadIdentityMapper: trafficpermissionsmapper.New(),
	}
}

// RegisterControllers registers controllers for the auth types with
// the given controller manager.
func RegisterControllers(mgr *controller.Manager, deps ControllerDependencies) {
	controllers.Register(mgr, deps)
}
