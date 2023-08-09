// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusConditionValid = "valid"

	OKReason = "Ok"

	MissingParentRefReason  = "MissingParentRef"
	MissingBackendRefReason = "MissingBackendRef"

	ParentRefOutsideMeshReason  = "ParentRefOutsideMesh"
	BackendRefOutsideMeshReason = "BackendRefOutsideMesh"

	UnknownParentRefPortReason  = "UnknownParentRefPort"
	UnknownBackendRefPortReason = "UnknownBackendRefPort"

	ConflictNotBoundToParentRefReason = "ConflictNotBoundToParentRef"
)

var (
	ConditionOK = &pbresource.Condition{
		Type:   StatusConditionValid,
		State:  pbresource.Condition_STATE_TRUE,
		Reason: OKReason,
		// TODO: needs message?
	}
)

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
		Type:   StatusConditionValid,
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
		Type:   StatusConditionValid,
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
		Type:   StatusConditionValid,
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

func ConditionConflictNotBoundToParentRef(ref *pbresource.Reference, port string, realType *pbresource.Type) *pbresource.Condition {
	return &pbresource.Condition{
		Type:   StatusConditionValid,
		State:  pbresource.Condition_STATE_FALSE,
		Reason: ConflictNotBoundToParentRefReason,
		Message: fmt.Sprintf(
			"Existing routes of type %q are bound to parent ref %q on port %q preventing this from binding",
			resource.TypeToString(realType),
			resource.ReferenceToString(ref),
			port,
		),
	}
}
