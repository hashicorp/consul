// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcclink

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey = "consul.io/hcc-link"

	StatusLinked                      = "linked"
	LinkedReason                      = "SUCCESS"
	DisabledReasonResourceAPIsEnabled = "DISABLED"

	LinkedMessageFormat                = "Successfully linked to cluster '%s'"
	DisabledResourceAPIsEnabledMessage = "Link is disabled because resource-apis are enabled"
)

var (
	ConditionDisabled = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  DisabledReasonResourceAPIsEnabled,
		Message: DisabledResourceAPIsEnabledMessage,
	}
)

func ConditionLinked(resourceId string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  LinkedReason,
		Message: fmt.Sprintf(LinkedMessageFormat, resourceId),
	}
}
