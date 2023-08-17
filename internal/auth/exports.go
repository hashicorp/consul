// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

// All we need is a resource controller that is already registered?
