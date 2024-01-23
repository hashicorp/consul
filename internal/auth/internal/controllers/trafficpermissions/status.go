// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey                                = "consul.io/traffic-permissions"
	StatusTrafficPermissionsComputed         = "Traffic permissions have been computed"
	StatusTrafficPermissionsNotComputed      = "Traffic permissions have not been computed"
	StatusTrafficPermissionsProcessed        = "Traffic Permission processed successfully"
	ConditionPermissionsAppliedMsg           = "Workload identity %s has new permissions"
	ConditionNoPermissionsMsg                = "Workload identity %s has no permissions"
	ConditionPermissionsFailedMsg            = "Unable to calculate new permission set for Workload identity %s"
	ConditionMissingSamenessGroupInPartition = "Missing Sameness Groups names in partition(%s) - %s"
)

func ConditionComputed(workloadIdentity string, isDefault bool) *pbresource.Condition {
	msgTpl := ConditionPermissionsAppliedMsg
	if isDefault {
		msgTpl = ConditionNoPermissionsMsg
	}
	return &pbresource.Condition{
		Type:    StatusTrafficPermissionsComputed,
		State:   pbresource.Condition_STATE_TRUE,
		Message: fmt.Sprintf(msgTpl, workloadIdentity),
	}
}

func ConditionComputedTrafficPermission() *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusTrafficPermissionsComputed,
		State:   pbresource.Condition_STATE_TRUE,
		Message: StatusTrafficPermissionsProcessed,
	}
}

func ConditionFailedToCompute(workloadIdentity string, trafficPermissions string, errDetail string) *pbresource.Condition {
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

func ConditionMissingSamenessGroup(partition string, missingSamenessGroups []string) *pbresource.Condition {
	sort.Strings(missingSamenessGroups)
	message := fmt.Sprintf(ConditionMissingSamenessGroupInPartition, partition, strings.Join(missingSamenessGroups, ","))
	return &pbresource.Condition{
		Type:    StatusTrafficPermissionsNotComputed,
		State:   pbresource.Condition_STATE_FALSE,
		Message: message,
	}
}
