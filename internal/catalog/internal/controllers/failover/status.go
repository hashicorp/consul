// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package failover

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ControllerID            = "consul.io/failover-policy"
	StatusConditionAccepted = "accepted"

	OKReason  = "Ok"
	OKMessage = "failover policy was accepted"

	MissingServiceReason  = "MissingService"
	MissingServiceMessage = "service for failover policy does not exist"

	UnknownPortReason        = "UnknownPort"
	UnknownPortMessagePrefix = "port is not defined on service: "

	MissingDestinationServiceReason        = "MissingDestinationService"
	MissingDestinationServiceMessagePrefix = "destination service for failover policy does not exist: "

	UnknownDestinationPortReason        = "UnknownDestinationPort"
	UnknownDestinationPortMessagePrefix = "port is not defined on destination service: "

	UsingMeshDestinationPortReason        = "UsingMeshDestinationPort"
	UsingMeshDestinationPortMessagePrefix = "port is a special unroutable mesh port on destination service: "

	MissingSamenessGroupReason        = "MissingSamenessGroup"
	MissingSamenessGroupMessagePrefix = "referenced sameness group does not exist: "

	ConflictDestinationPortReason        = "ConflictDestinationPort"
	ConflictDestinationPortMessagePrefix = "multiple configs found for port on destination service: "
)

var (
	ConditionOK = &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  OKReason,
		Message: OKMessage,
	}

	ConditionMissingService = &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  MissingServiceReason,
		Message: MissingServiceMessage,
	}
)

func ConditionUnknownPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  UnknownPortReason,
		Message: UnknownPortMessagePrefix + port + " on " + resource.ReferenceToString(ref),
	}
}

func ConditionMissingDestinationService(ref *pbresource.Reference) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  MissingDestinationServiceReason,
		Message: MissingDestinationServiceMessagePrefix + resource.ReferenceToString(ref),
	}
}

func ConditionUnknownDestinationPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  UnknownDestinationPortReason,
		Message: UnknownDestinationPortMessagePrefix + port + " on " + resource.ReferenceToString(ref),
	}
}

func ConditionUsingMeshDestinationPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  UnknownDestinationPortReason,
		Message: UnknownDestinationPortMessagePrefix + port + " on " + resource.ReferenceToString(ref),
	}
}

func ConditionMissingSamenessGroup(ref *pbresource.Reference) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  MissingSamenessGroupReason,
		Message: MissingSamenessGroupMessagePrefix + resource.ReferenceToString(ref),
	}
}

func ConditionConflictDestinationPort(ref *pbresource.Reference, port *pbcatalog.ServicePort) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  ConflictDestinationPortReason,
		Message: ConflictDestinationPortMessagePrefix + port.ToPrintableString() + " on " + resource.ReferenceToString(ref),
	}
}
