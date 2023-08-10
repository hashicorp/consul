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

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion

	// Resource Kind Names.

	ProxyConfigurationKind         = types.ProxyConfigurationKind
	UpstreamsKind                  = types.UpstreamsKind
	UpstreamsConfigurationKind     = types.UpstreamsConfigurationKind
	ProxyStateKind                 = types.ProxyStateTemplateKind
	HTTPRouteKind                  = types.HTTPRouteKind
	GRPCRouteKind                  = types.GRPCRouteKind
	TCPRouteKind                   = types.TCPRouteKind
	DestinationPolicyKind          = types.DestinationPolicyKind
	ComputedRoutesKind             = types.ComputedRoutesKind
	ProxyStateTemplateV1Alpha1Type = types.ProxyStateTemplateV1Alpha1Type

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
	ProxyStateTemplateType                      = types.ProxyStateTemplateV1Alpha1Type

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
