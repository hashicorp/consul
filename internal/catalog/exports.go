// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalog

import (
	"github.com/hashicorp/consul/internal/catalog/internal/controllers"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/endpoints"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/failover"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	// Controller Statuses
	NodeHealthStatusKey              = nodehealth.StatusKey
	NodeHealthStatusConditionHealthy = nodehealth.StatusConditionHealthy
	NodeHealthConditions             = nodehealth.Conditions

	WorkloadHealthStatusKey              = workloadhealth.ControllerID
	WorkloadHealthStatusConditionHealthy = workloadhealth.StatusConditionHealthy
	WorkloadHealthConditions             = workloadhealth.WorkloadConditions
	WorkloadAndNodeHealthConditions      = workloadhealth.NodeAndWorkloadConditions

	EndpointsStatusKey                       = endpoints.ControllerID
	EndpointsStatusConditionEndpointsManaged = endpoints.StatusConditionEndpointsManaged
	EndpointsStatusConditionManaged          = endpoints.ConditionManaged
	EndpointsStatusConditionUnmanaged        = endpoints.ConditionUnmanaged
	StatusConditionBoundIdentities           = endpoints.StatusConditionBoundIdentities
	StatusReasonWorkloadIdentitiesFound      = endpoints.StatusReasonWorkloadIdentitiesFound
	StatusReasonNoWorkloadIdentitiesFound    = endpoints.StatusReasonNoWorkloadIdentitiesFound

	FailoverStatusKey                                              = failover.ControllerID
	FailoverStatusConditionAccepted                                = failover.StatusConditionAccepted
	FailoverStatusConditionAcceptedOKReason                        = failover.OKReason
	FailoverStatusConditionAcceptedMissingServiceReason            = failover.MissingServiceReason
	FailoverStatusConditionAcceptedUnknownPortReason               = failover.UnknownPortReason
	FailoverStatusConditionAcceptedMissingDestinationServiceReason = failover.MissingDestinationServiceReason
	FailoverStatusConditionAcceptedUnknownDestinationPortReason    = failover.UnknownDestinationPortReason
	FailoverStatusConditionAcceptedUsingMeshDestinationPortReason  = failover.UsingMeshDestinationPortReason
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

// SimplifyFailoverPolicy fully populates the PortConfigs map and clears the
// Configs map using the provided Service.
func SimplifyFailoverPolicy(svc *pbcatalog.Service, failover *pbcatalog.FailoverPolicy) *pbcatalog.FailoverPolicy {
	return types.SimplifyFailoverPolicy(svc, failover)
}

// GetBoundIdentities returns the unique list of workload identity references
// encoded into a data-bearing status condition on a Service resource by the
// endpoints controller.
//
// This allows a controller to skip watching ServiceEndpoints (which is
// expensive) to discover this data.
func GetBoundIdentities(res *pbresource.Resource) []string {
	return endpoints.GetBoundIdentities(res)
}

// ValidateLocalServiceRefNoSection ensures the following:
//
// - ref is non-nil
// - type is ServiceType
// - section is empty
// - tenancy is set and partition/namespace are both non-empty
// - peer_name must be "local"
//
// Each possible validation error is wrapped in the wrapErr function before
// being collected in a multierror.Error.
func ValidateLocalServiceRefNoSection(ref *pbresource.Reference, wrapErr func(error) error) error {
	return types.ValidateLocalServiceRefNoSection(ref, wrapErr)
}

// ValidateSelector ensures that the selector has at least one exact or prefix
// match constraint, and that if a filter is present it is valid.
//
// The selector can be nil, and have zero exact/prefix matches if allowEmpty is
// set to true.
func ValidateSelector(sel *pbcatalog.WorkloadSelector, allowEmpty bool) error {
	return types.ValidateSelector(sel, allowEmpty)
}

func ValidatePortName(id string) error {
	return types.ValidatePortName(id)
}

func ValidateServicePortID(id string) error {
	return types.ValidateServicePortID(id)
}

func IsValidUnixSocketPath(host string) bool {
	return types.IsValidUnixSocketPath(host)
}

func ValidateProtocol(protocol pbcatalog.Protocol) error {
	return types.ValidateProtocol(protocol)
}
