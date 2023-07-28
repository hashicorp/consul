package status

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusConditionMeshDestination = "MeshDestination"

	StatusReasonNonMeshDestination = "MeshPortProtocolNotFound"
	StatusReasonMeshDestination    = "MeshPortProtocolFound"

	StatusConditionDestinationExists = "DestinationExists"

	StatusReasonDestinationServiceNotFound = "ServiceNotFound"
	StatusReasonDestinationServiceFound    = "ServiceFound"
)

func ConditionNonMeshDestination(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionMeshDestination,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonNonMeshDestination,
		Message: fmt.Sprintf("service %q cannot be referenced as a Destination because it's not mesh-enabled.", serviceRef),
	}
}

func ConditionMeshDestination(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionMeshDestination,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonMeshDestination,
		Message: fmt.Sprintf("service %q is on the mesh.", serviceRef),
	}
}

func ConditionDestinationServiceNotFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationExists,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonDestinationServiceNotFound,
		Message: fmt.Sprintf("service %q does not exist.", serviceRef),
	}
}

func ConditionDestinationServiceFound(serviceRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionDestinationExists,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonDestinationServiceFound,
		Message: fmt.Sprintf("service %q exists.", serviceRef),
	}
}
