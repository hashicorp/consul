// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey                           = "consul.io/traffic-permissions"
	StatusTrafficPermissionsComputed    = "Traffic permissions have been computed"
	StatusTrafficPermissionsNotComputed = "Traffic permissions have been computed"
	ConditionPermissionsAppliedMsg      = "Workload identity %s has new permission set"
	ConditionPermissionsFailedMsg       = "Unable to calculate new permission set for Workload identity %s"
)

var (
	ConditionComputed = func(workloadIdentity string) *pbresource.Condition {
		return &pbresource.Condition{
			Type:    StatusTrafficPermissionsComputed,
			State:   pbresource.Condition_STATE_TRUE,
			Message: fmt.Sprintf(ConditionPermissionsAppliedMsg, workloadIdentity),
		}
	}
	ConditionFailedToCompute = func(workloadIdentity string, trafficPermissions string, errDetail string) *pbresource.Condition {
		message := fmt.Sprintf(ConditionPermissionsFailedMsg, workloadIdentity)
		if len(trafficPermissions) > 0 {
			message = message + fmt.Sprintf(", traffic permission %s cannot be computed", trafficPermissions)
		}
		if len(errDetail) > 0 {
			message = message + fmt.Sprintf(", error details: %s", errDetail)
		}
		return &pbresource.Condition{
			Type:    StatusTrafficPermissionsNotComputed,
			State:   pbresource.Condition_STATE_FALSE,
			Message: message,
		}
	}
)
