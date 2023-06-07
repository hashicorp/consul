// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodehealth

import (
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey              = "consul.io/node-health"
	StatusConditionHealthy = "healthy"

	NodeHealthyMessage   = "All node health checks are passing"
	NodeUnhealthyMessage = "One or more node health checks are not passing"
)

var (
	ConditionPassing = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  pbcatalog.Health_HEALTH_PASSING.String(),
		Message: NodeHealthyMessage,
	}

	ConditionWarning = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_WARNING.String(),
		Message: NodeUnhealthyMessage,
	}

	ConditionCritical = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_CRITICAL.String(),
		Message: NodeUnhealthyMessage,
	}

	ConditionMaintenance = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_MAINTENANCE.String(),
		Message: NodeUnhealthyMessage,
	}

	Conditions = map[pbcatalog.Health]*pbresource.Condition{
		pbcatalog.Health_HEALTH_PASSING:     ConditionPassing,
		pbcatalog.Health_HEALTH_WARNING:     ConditionWarning,
		pbcatalog.Health_HEALTH_CRITICAL:    ConditionCritical,
		pbcatalog.Health_HEALTH_MAINTENANCE: ConditionMaintenance,
	}
)
