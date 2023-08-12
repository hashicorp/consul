// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mesh

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// API Group Information

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion

	// Resource Kind Names.

	ProxyConfigurationKind     = types.ProxyConfigurationKind
	UpstreamsKind              = types.UpstreamsKind
	UpstreamsConfigurationKind = types.UpstreamsConfigurationKind
	ProxyStateKind             = types.ProxyStateTemplateKind
	HTTPRouteKind              = types.HTTPRouteKind
	GRPCRouteKind              = types.GRPCRouteKind
	TCPRouteKind               = types.TCPRouteKind
	DestinationPolicyKind      = types.DestinationPolicyKind
	ComputedRoutesKind         = types.ComputedRoutesKind

	// Resource Types for the v1alpha1 version.

	ProxyConfigurationV1Alpha1Type              = types.ProxyConfigurationV1Alpha1Type
	UpstreamsV1Alpha1Type                       = types.UpstreamsV1Alpha1Type
	UpstreamsConfigurationV1Alpha1Type          = types.UpstreamsConfigurationV1Alpha1Type
	ProxyStateTemplateConfigurationV1Alpha1Type = types.ProxyStateTemplateV1Alpha1Type
	HTTPRouteV1Alpha1Type                       = types.HTTPRouteV1Alpha1Type
	GRPCRouteV1Alpha1Type                       = types.GRPCRouteV1Alpha1Type
	TCPRouteV1Alpha1Type                        = types.TCPRouteV1Alpha1Type
	DestinationPolicyV1Alpha1Type               = types.DestinationPolicyV1Alpha1Type
	ComputedRoutesV1Alpha1Type                  = types.ComputedRoutesV1Alpha1Type

	// Resource Types for the latest version.

	ProxyConfigurationType              = types.ProxyConfigurationType
	UpstreamsType                       = types.UpstreamsType
	UpstreamsConfigurationType          = types.UpstreamsConfigurationType
	ProxyStateTemplateConfigurationType = types.ProxyStateTemplateType
	HTTPRouteType                       = types.HTTPRouteType
	GRPCRouteType                       = types.GRPCRouteType
	TCPRouteType                        = types.TCPRouteType
	DestinationPolicyType               = types.DestinationPolicyType
	ComputedRoutesType                  = types.ComputedRoutesType
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
