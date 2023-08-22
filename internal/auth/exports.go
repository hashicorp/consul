// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package auth

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers"
	"github.com/hashicorp/consul/internal/auth/internal/mappers/trafficpermissionsmapper"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// API Group Information

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion

	// Resource Kind Names.

	WorkloadIdentity = types.WorkloadIdentityKind

	// Resource Types for the v1alpha1 version.

	WorkloadIdentityV1Alpha1Type = types.WorkloadIdentityV1Alpha1Type

	// Resource Types for the latest version.

	WorkloadIdentityType = types.WorkloadIdentityType

	// Controller Statuses
	// TODO
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

type ControllerDependencies = controllers.Dependencies

func DefaultControllerDependencies() ControllerDependencies {
	return ControllerDependencies{
		ComputedTrafficPermissionsMapper: trafficpermissionsmapper.New(),
	}
}

// RegisterControllers registers controllers for the auth types with
// the given controller manager.
func RegisterControllers(mgr *controller.Manager, deps ControllerDependencies) {
	controllers.Register(mgr, deps)
}
