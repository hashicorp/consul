// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey                        = "consul.io/traffic-permissions"
	StatusTrafficPermissionsComputed = "Traffic permissions have been computed"
	ConditionPermissionsAppliedMsg   = "Workload Identity %s has new permission set"
)

var (
	ConditionComputed = func(workloadIdentity string) *pbresource.Condition {
		return &pbresource.Condition{
			Type:    StatusTrafficPermissionsComputed,
			State:   pbresource.Condition_STATE_TRUE,
			Message: fmt.Sprintf(ConditionPermissionsAppliedMsg, workloadIdentity),
		}
	}
)
