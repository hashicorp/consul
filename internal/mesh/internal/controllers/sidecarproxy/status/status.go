// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package status

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusConditionDestinationsAccepted = "DestinationsAccepted"

	StatusReasonMeshProtocolNotFound                  = "MeshPortProtocolNotFound"
	StatusReasonDestinationPortNotFound               = "DestinationPortNotFound"
	StatusReasonMeshProtocolDestinationPort           = "DestinationWithMeshPortProtocol"
	StatusReasonDestinationServiceNotFound            = "ServiceNotFound"
	StatusReasonDestinationComputedRoutesNotFound     = "ComputedRoutesNotFound"
	StatusReasonDestinationComputedRoutesPortNotFound = "ComputedRoutesPortNotFound"
	StatusReasonAllDestinationsValid                  = "AllDestinationsValid"
)

func ConditionMeshProtocolNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonMeshProtocolNotFound,
		Message: fmt.Sprintf("service %q cannot be referenced as a Destination because it's not mesh-enabled.", serviceRef),
	}
}

func ConditionDestinationPortNotFound(serviceRef string, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationPortNotFound,
		Message: fmt.Sprintf("service %q does not have desired port %q.", serviceRef, port),
	}
}

func ConditionAllDestinationsValid() *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonAllDestinationsValid,
		Message: fmt.Sprintf("all destinations are valid."),
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

func ConditionDestinationComputedRoutesPortNotFound(serviceRef, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationsAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationComputedRoutesPortNotFound,
		Message: fmt.Sprintf("computed routes %q does not exist for port %q.", serviceRef, port),
	}
}
