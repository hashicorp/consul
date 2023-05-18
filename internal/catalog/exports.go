// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalog

import (
	"github.com/hashicorp/consul/internal/catalog/internal/controllers"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// API Group Information

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion

	// Resource Kind Names.

	WorkloadKind         = types.WorkloadKind
	ServiceKind          = types.ServiceKind
	ServiceEndpointsKind = types.ServiceEndpointsKind
	VirtualIPsKind       = types.VirtualIPsKind
	NodeKind             = types.NodeKind
	HealthStatusKind     = types.HealthStatusKind
	HealthChecksKind     = types.HealthChecksKind
	DNSPolicyKind        = types.DNSPolicyKind

	// Resource Types for the v1alpha1 version.

	WorkloadV1Alpha1Type         = types.WorkloadV1Alpha1Type
	ServiceV1Alpha1Type          = types.ServiceV1Alpha1Type
	ServiceEndpointsV1Alpha1Type = types.ServiceEndpointsV1Alpha1Type
	VirtualIPsV1Alpha1Type       = types.VirtualIPsV1Alpha1Type
	NodeV1Alpha1Type             = types.NodeV1Alpha1Type
	HealthStatusV1Alpha1Type     = types.HealthStatusV1Alpha1Type
	HealthChecksV1Alpha1Type     = types.HealthChecksV1Alpha1Type
	DNSPolicyV1Alpha1Type        = types.DNSPolicyV1Alpha1Type
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

// RegisterControllers registers controllers for the catalog types with
// the given controller Manager.
func RegisterControllers(mgr *controller.Manager) {
	controllers.Register(mgr)
}
