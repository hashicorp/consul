// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodehealth

const (
	StatusKey              = "consul.io/node-health"
	StatusConditionHealthy = "healthy"

	NodeHealthyMessage   = "All node health checks are passing"
	NodeUnhealthyMessage = "One or more node health checks are not passing"
)
