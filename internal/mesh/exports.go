// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mesh

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// API Group Information

	APIGroup       = types.GroupName
	VersionV2Beta1 = types.VersionV2beta1
	CurrentVersion = types.CurrentVersion

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
	ProxyStateTemplateKind     = types.ProxyStateTemplateKind

	// Resource Types for the v1alpha1 version.

	ProxyConfigurationV2Beta1Type              = types.ProxyConfigurationV2Beta1Type
	UpstreamsV2Beta1Type                       = types.UpstreamsV2Beta1Type
	UpstreamsConfigurationV2Beta1Type          = types.UpstreamsConfigurationV2Beta1Type
	ProxyStateTemplateConfigurationV2Beta1Type = types.ProxyStateTemplateV2Beta1Type
	HTTPRouteV2Beta1Type                       = types.HTTPRouteV2Beta1Type
	GRPCRouteV2Beta1Type                       = types.GRPCRouteV2Beta1Type
	TCPRouteV2Beta1Type                        = types.TCPRouteV2Beta1Type
	DestinationPolicyV2Beta1Type               = types.DestinationPolicyV2Beta1Type
	ComputedRoutesV2Beta1Type                  = types.ComputedRoutesV2Beta1Type
	ProxyStateTemplateV1AlphaType              = types.ProxyStateTemplateV2Beta1Type

	// Resource Types for the latest version.

	ProxyConfigurationType     = types.ProxyConfigurationType
	UpstreamsType              = types.UpstreamsType
	UpstreamsConfigurationType = types.UpstreamsConfigurationType
	ProxyStateTemplateType     = types.ProxyStateTemplateType
	HTTPRouteType              = types.HTTPRouteType
	GRPCRouteType              = types.GRPCRouteType
	TCPRouteType               = types.TCPRouteType
	DestinationPolicyType      = types.DestinationPolicyType
	ComputedRoutesType         = types.ComputedRoutesType

	// Controller statuses.

	// Sidecar-proxy controller.
	SidecarProxyStatusKey                                  = sidecarproxy.ControllerName
	SidecarProxyStatusConditionMeshDestination             = status.StatusConditionDestinationAccepted
	SidecarProxyStatusReasonNonMeshDestination             = status.StatusReasonMeshProtocolNotFound
	SidecarProxyStatusReasonMeshDestination                = status.StatusReasonMeshProtocolFound
	SidecarProxyStatusReasonDestinationServiceNotFound     = status.StatusReasonDestinationServiceNotFound
	SidecarProxyStatusReasonDestinationServiceFound        = status.StatusReasonDestinationServiceFound
	SidecarProxyStatusReasonMeshProtocolDestinationPort    = status.StatusReasonMeshProtocolDestinationPort
	SidecarProxyStatusReasonNonMeshProtocolDestinationPort = status.StatusReasonNonMeshProtocolDestinationPort
)

// RegisterTypes adds all resource types within the "mesh" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

// RegisterControllers registers controllers for the mesh types with
// the given controller Manager.
func RegisterControllers(mgr *controller.Manager, deps ControllerDependencies) {
	controllers.Register(mgr, deps)
}

type TrustDomainFetcher = sidecarproxy.TrustDomainFetcher

type ControllerDependencies = controllers.Dependencies
