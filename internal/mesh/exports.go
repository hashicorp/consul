// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mesh

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

var (
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

	// Routes controller
	RoutesStatusKey                                                = routes.StatusKey
	RoutesStatusConditionAccepted                                  = routes.StatusConditionAccepted
	RoutesStatusConditionAcceptedMissingParentRefReason            = routes.MissingParentRefReason
	RoutesStatusConditionAcceptedMissingBackendRefReason           = routes.MissingBackendRefReason
	RoutesStatusConditionAcceptedParentRefOutsideMeshReason        = routes.ParentRefOutsideMeshReason
	RoutesStatusConditionAcceptedBackendRefOutsideMeshReason       = routes.BackendRefOutsideMeshReason
	RoutesStatusConditionAcceptedParentRefUsingMeshPortReason      = routes.ParentRefUsingMeshPortReason
	RoutesStatusConditionAcceptedBackendRefUsingMeshPortReason     = routes.BackendRefUsingMeshPortReason
	RoutesStatusConditionAcceptedUnknownParentRefPortReason        = routes.UnknownParentRefPortReason
	RoutesStatusConditionAcceptedUnknownBackendRefPortReason       = routes.UnknownBackendRefPortReason
	RoutesStatusConditionAcceptedConflictNotBoundToParentRefReason = routes.ConflictNotBoundToParentRefReason
)

const (
	// Important constants

	NullRouteBackend = types.NullRouteBackend
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
