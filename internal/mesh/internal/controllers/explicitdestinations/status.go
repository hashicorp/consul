// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package explicitdestinations

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusConditionDestinationsAccepted = "DestinationsAccepted"

	StatusReasonMeshProtocolNotFound = "MeshPortProtocolNotFound"
	StatusReasonMeshProtocolFound    = "AllDestinationServicesValid"

	StatusReasonMeshProtocolDestinationPort = "DestinationWithMeshPortProtocol"

	StatusReasonDestinationServiceNotFound  = "ServiceNotFound"
	StatusReasonDestinationServiceReadError = "ServiceReadError"

	StatusReasonDestinationComputedRoutesNotFound  = "ComputedRoutesNotFound"
	StatusReasonDestinationComputedRoutesReadError = "ComputedRoutesReadError"

	StatusReasonDestinationComputedRoutesPortNotFound = "ComputedRoutesPortNotFound"

	StatusConditionConflictFound         = "ConflictFound"
	StatusReasonDuplicateListenAddress   = "ConflictingListenAddress"
	StatusReasonNoDuplicateListenAddress = "AllListenAddressesAreUnique"
)

var ConditionConflictNotFound = &pbresource.Condition{
	Type:    StatusConditionConflictFound,
	State:   pbresource.Condition_STATE_FALSE,
	Reason:  StatusReasonNoDuplicateListenAddress,
	Message: "All mapper have unique listen addresses.",
}

func ConditionConflictFound(workloadID *pbresource.ID) *pbresource.Condition {
	return &pbresource.Condition{
		Type:   StatusConditionConflictFound,
		State:  pbresource.Condition_STATE_TRUE,
		Reason: StatusReasonDuplicateListenAddress,
		Message: fmt.Sprintf("Another Destinations resource selecting workload %q configures the same listen address as one of the mapper in this resource. "+
			"This resource will be skipped.", resource.IDToString(workloadID)),
	}
}

func ConditionMeshProtocolNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonMeshProtocolNotFound,
		Message: fmt.Sprintf("service %q cannot be referenced as a Destination because it's not mesh-enabled.", serviceRef),
	}
}

func ConditionDestinationsAccepted() *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonMeshProtocolFound,
		Message: "all destination services are valid.",
	}
}

func ConditionDestinationServiceNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationServiceNotFound,
		Message: fmt.Sprintf("service %q does not exist.", serviceRef),
	}
}

func ConditionDestinationServiceReadError(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationServiceReadError,
		Message: fmt.Sprintf("error reading service %q", serviceRef),
	}
}

func ConditionMeshProtocolDestinationPort(serviceRef, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonMeshProtocolDestinationPort,
		Message: fmt.Sprintf("destination port %q for service %q has PROTOCOL_MESH which is unsupported for destination services", port, serviceRef),
	}
}

func ConditionDestinationComputedRoutesNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationComputedRoutesNotFound,
		Message: fmt.Sprintf("computed routes %q does not exist.", serviceRef),
	}
}

func ConditionDestinationComputedRoutesReadErr(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationComputedRoutesReadError,
		Message: fmt.Sprintf("error reading computed routes for %q service.", serviceRef),
	}
}

func ConditionDestinationComputedRoutesPortNotFound(serviceRef, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationComputedRoutesPortNotFound,
		Message: fmt.Sprintf("computed routes %q does not exist for port %q.", serviceRef, port),
	}
}
