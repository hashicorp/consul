// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package status

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusConditionDestinationAccepted = "DestinationAccepted"

	StatusReasonMeshProtocolNotFound = "MeshPortProtocolNotFound"
	StatusReasonMeshProtocolFound    = "MeshPortProtocolFound"

	StatusReasonMeshProtocolDestinationPort    = "DestinationWithMeshPortProtocol"
	StatusReasonNonMeshProtocolDestinationPort = "DestinationWithNonMeshPortProtocol"

	StatusReasonDestinationServiceNotFound = "ServiceNotFound"
	StatusReasonDestinationServiceFound    = "ServiceFound"

	StatusReasonDestinationComputedRoutesNotFound = "ComputedRoutesNotFound"
	StatusReasonDestinationComputedRoutesFound    = "ComputedRoutesFound"

	StatusReasonDestinationComputedRoutesPortNotFound = "ComputedRoutesPortNotFound"
	StatusReasonDestinationComputedRoutesPortFound    = "ComputedRoutesPortFound"
)

func ConditionMeshProtocolNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonMeshProtocolNotFound,
		Message: fmt.Sprintf("service %q cannot be referenced as a Destination because it's not mesh-enabled.", serviceRef),
	}
}

func ConditionMeshProtocolFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonMeshProtocolFound,
		Message: fmt.Sprintf("service %q is on the mesh.", serviceRef),
	}
}

func ConditionDestinationServiceNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationServiceNotFound,
		Message: fmt.Sprintf("service %q does not exist.", serviceRef),
	}
}

func ConditionDestinationServiceFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonDestinationServiceFound,
		Message: fmt.Sprintf("service %q exists.", serviceRef),
	}
}

func ConditionMeshProtocolDestinationPort(serviceRef, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonMeshProtocolDestinationPort,
		Message: fmt.Sprintf("destination port %q for service %q has PROTOCOL_MESH which is unsupported for destination services", port, serviceRef),
	}
}

func ConditionNonMeshProtocolDestinationPort(serviceRef, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonNonMeshProtocolDestinationPort,
		Message: fmt.Sprintf("destination port %q for service %q has a non-mesh protocol", port, serviceRef),
	}
}

func ConditionDestinationComputedRoutesNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationComputedRoutesNotFound,
		Message: fmt.Sprintf("computed routes %q does not exist.", serviceRef),
	}
}

func ConditionDestinationComputedRoutesFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonDestinationComputedRoutesNotFound,
		Message: fmt.Sprintf("computed routes %q exists.", serviceRef),
	}
}

func ConditionDestinationComputedRoutesPortNotFound(serviceRef, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationComputedRoutesPortNotFound,
		Message: fmt.Sprintf("computed routes %q does not exist for port %q.", serviceRef, port),
	}
}

func ConditionDestinationComputedRoutesPortFound(serviceRef, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonDestinationComputedRoutesPortNotFound,
		Message: fmt.Sprintf("computed routes %q exists for port %q.", serviceRef, port),
	}
}
