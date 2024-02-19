// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	catalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey               = "consul.io/routes-controller"
	StatusConditionAccepted = "accepted"

	// conditions on xRoutes

	XRouteOKReason  = "Ok"
	XRouteOKMessage = "xRoute was accepted"

	MissingParentRefReason  = "MissingParentRef"
	MissingBackendRefReason = "MissingBackendRef"

	ParentRefOutsideMeshReason  = "ParentRefOutsideMesh"
	BackendRefOutsideMeshReason = "BackendRefOutsideMesh"

	ParentRefUsingMeshPortReason  = "ParentRefUsingMeshPort"
	BackendRefUsingMeshPortReason = "BackendRefUsingMeshPort"

	UnknownParentRefPortReason    = "UnknownParentRefPort"
	UnknownBackendRefPortReason   = "UnknownBackendRefPort"
	UnknownDestinationPortReason  = "UnknownDestinationPort"
	ConflictParentRefPortReason   = "ConflictParentRefPort"
	ConflictBackendRefPortReason  = "ConflictBackendRefPort"
	ConflictDestinationPortReason = "ConflictDestinationPort"

	ConflictNotBoundToParentRefReason = "ConflictNotBoundToParentRef"

	DestinationServiceNotFoundReason = "DestinationServiceNotFound"
)

var (
	ConditionXRouteOK = &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  XRouteOKReason,
		Message: XRouteOKMessage,
	}
)

func ConditionParentRefUsingMeshPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return conditionRefUsingMeshPort(ref, port, false)
}

func ConditionBackendRefUsingMeshPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return conditionRefUsingMeshPort(ref, port, true)
}

func conditionRefUsingMeshPort(ref *pbresource.Reference, port string, forBackend bool) *pbresource.Condition {
	reason := ParentRefUsingMeshPortReason
	short := "parent"
	if forBackend {
		reason = BackendRefUsingMeshPortReason
		short = "backend"
	}
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: reason,
		Message: fmt.Sprintf(
			"service for %s ref %q uses port %q which is a special unroutable mesh port",
			short,
			resource.ReferenceToString(ref),
			port,
		),
	}
}

func ConditionMissingParentRef(ref *pbresource.Reference) *pbresource.Condition {
	return conditionMissingRef(ref, false)
}

func ConditionMissingBackendRef(ref *pbresource.Reference) *pbresource.Condition {
	return conditionMissingRef(ref, true)
}

func conditionMissingRef(ref *pbresource.Reference, forBackend bool) *pbresource.Condition {
	reason := MissingParentRefReason
	short := "parent"
	if forBackend {
		reason = MissingBackendRefReason
		short = "backend"
	}
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: reason,
		Message: fmt.Sprintf(
			"service for %s ref %q does not exist",
			short,
			resource.ReferenceToString(ref),
		),
	}
}

func ConditionParentRefOutsideMesh(ref *pbresource.Reference) *pbresource.Condition {
	return conditionRefOutsideMesh(ref, false)
}

func ConditionBackendRefOutsideMesh(ref *pbresource.Reference) *pbresource.Condition {
	return conditionRefOutsideMesh(ref, true)
}

func conditionRefOutsideMesh(ref *pbresource.Reference, forBackend bool) *pbresource.Condition {
	reason := ParentRefOutsideMeshReason
	short := "parent"
	if forBackend {
		reason = BackendRefOutsideMeshReason
		short = "backend"
	}
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: reason,
		Message: fmt.Sprintf(
			"service for %s ref %q does not expose a mesh port",
			short,
			resource.ReferenceToString(ref),
		),
	}
}

func ConditionUnknownParentRefPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return conditionUnknownRefPort(ref, port, false)
}

func ConditionUnknownBackendRefPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return conditionUnknownRefPort(ref, port, true)
}

func conditionUnknownRefPort(ref *pbresource.Reference, port string, forBackend bool) *pbresource.Condition {
	reason := UnknownParentRefPortReason
	short := "parent"
	if forBackend {
		reason = UnknownBackendRefPortReason
		short = "backend"
	}
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: reason,
		Message: fmt.Sprintf(
			"service for %s ref %q does not expose port %q",
			short,
			resource.ReferenceToString(ref),
			port,
		),
	}
}

func ConditionConflictParentRefPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return conditionConflictRefPort(ref, port, false)
}

func ConditionConflictBackendRefPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return conditionConflictRefPort(ref, port, true)
}

func conditionConflictRefPort(ref *pbresource.Reference, port string, forBackend bool) *pbresource.Condition {
	reason := ConflictParentRefPortReason
	short := "parent"
	if forBackend {
		reason = ConflictBackendRefPortReason
		short = "backend"
	}
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: reason,
		Message: fmt.Sprintf(
			"multiple %s refs found for service %q on target port %q",
			short,
			resource.ReferenceToString(ref),
			port,
		),
	}
}

func ConditionConflictNotBoundToParentRef(ref *pbresource.Reference, port string, realType *pbresource.Type) *pbresource.Condition {
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: ConflictNotBoundToParentRefReason,
		Message: fmt.Sprintf(
			"existing routes of type %q are bound to parent ref %q on port %q preventing this from binding",
			resource.TypeToString(realType),
			resource.ReferenceToString(ref),
			port,
		),
	}
}

func ConditionDestinationServiceNotFound(serviceRef *pbresource.Reference) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  DestinationServiceNotFoundReason,
		Message: fmt.Sprintf("service %q does not exist.", resource.ReferenceToString(serviceRef)),
	}
}

func ConditionUnknownDestinationPort(serviceRef *pbresource.Reference, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: UnknownDestinationPortReason,
		Message: fmt.Sprintf(
			"port is not defined on service: %s on %s",
			port,
			resource.ReferenceToString(serviceRef),
		),
	}
}

func ConditionConflictDestinationPort(serviceRef *pbresource.Reference, port *catalog.ServicePort) *pbresource.Condition {
	return &pbresource.Condition{
		Type:   StatusConditionAccepted,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: ConflictDestinationPortReason,
		Message: fmt.Sprintf(
			"multiple configs found for port on destination service: %s on %s",
			port.ToPrintableString(),
			resource.ReferenceToString(serviceRef),
		),
	}
}
