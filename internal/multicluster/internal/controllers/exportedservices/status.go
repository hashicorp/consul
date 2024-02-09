// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	statusKey = "consul.io/exported-services"

	statusExportedServicesComputed = "ExportedServicesComputed"
	statusMissingSamenessGroups    = "MissingSamenessGroups"

	msgExportedServicesComputed = "Exported services have been computed"
)

func conditionComputed() *pbresource.Condition {
	return &pbresource.Condition{
		Type:    statusExportedServicesComputed,
		State:   pbresource.Condition_STATE_TRUE,
		Message: msgExportedServicesComputed,
	}
}

func conditionNotComputed(message string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    statusExportedServicesComputed,
		State:   pbresource.Condition_STATE_FALSE,
		Message: message,
	}
}

func conditionMissingSamenessGroups(message string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    statusMissingSamenessGroups,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  "MissingSamenessGroups",
		Message: message,
	}
}
