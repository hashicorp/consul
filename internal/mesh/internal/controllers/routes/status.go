// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey              = "consul.io/routes-controller"
	StatusConditionHealthy = "healthy"

	MeshConfigHealthyMessage         = "Routing information is valid"
	MeshConfigUnhealthyMessagePrefix = "Routing information is not valid: "
)

var (
	ConditionMeshPassing = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  OKReason,
		Message: MeshConfigHealthyMessage,
	}
)

func ConditionMeshError(reason, message string) *pbresource.Condition {
	if reason == "" {
		panic("reason is required")
	}
	if message == "" {
		panic("message is required")
	}
	return &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  reason,
		Message: MeshConfigUnhealthyMessagePrefix + message,
	}
}
