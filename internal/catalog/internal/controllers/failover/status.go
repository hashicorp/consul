// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package failover

import (
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey              = "consul.io/failover-policy"
	StatusConditionHealthy = "healthy"

	OKReason = "Ok"

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
)

var (
	ConditionOK = &pbresource.Condition{
		Type:   StatusConditionHealthy,
		State:  pbresource.Condition_STATE_TRUE,
		Reason: OKReason,
		// TODO: needs message?
	}

	ConditionMissingService = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  MissingServiceReason,
		Message: MissingServiceMessage,
	}
)

func ConditionUnknownPort(port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  UnknownPortReason,
		Message: UnknownPortMessagePrefix + port,
	}
}

func ConditionMissingDestinationService(ref *pbresource.Reference) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  MissingDestinationServiceReason,
		Message: MissingDestinationServiceMessagePrefix + resource.ReferenceToString(ref),
	}
}

func ConditionUnknownDestinationPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  UnknownDestinationPortReason,
		Message: UnknownDestinationPortMessagePrefix + port + " on " + resource.ReferenceToString(ref),
	}
}

func ConditionUsingMeshDestinationPort(ref *pbresource.Reference, port string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  UnknownDestinationPortReason,
		Message: UnknownDestinationPortMessagePrefix + port + " on " + resource.ReferenceToString(ref),
	}
}
