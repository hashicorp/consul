// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package link

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey = "consul.io/hcp/link"

	StatusLinked                         = "linked"
	LinkedReason                         = "SUCCESS"
	FailedReason                         = "FAILED"
	DisabledReasonV2ResourcesUnsupported = "DISABLED_V2_RESOURCES_UNSUPPORTED"

	LinkedMessageFormat                = "Successfully linked to cluster '%s'"
	FailedMessage                      = "Failed to link to HCP"
	DisabledResourceAPIsEnabledMessage = "Link is disabled because resource-apis are enabled"
)

var (
	ConditionDisabled = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  DisabledReasonV2ResourcesUnsupported,
		Message: DisabledResourceAPIsEnabledMessage,
	}
	ConditionFailed = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  FailedReason,
		Message: FailedMessage,
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

func IsLinked(res *pbresource.Resource) (linked bool, reason string) {
	if !resource.EqualType(res.GetId().GetType(), pbhcp.LinkType) {
		return false, "resource is not hcp.Link type"
	}

	linkStatus, ok := res.GetStatus()[StatusKey]
	if !ok {
		return false, "link status not set"
	}

	for _, cond := range linkStatus.GetConditions() {
		if cond.Type == StatusLinked && cond.GetState() == pbresource.Condition_STATE_TRUE {
			return true, ""
		}
	}
	return false, "link status does not include positive linked condition"
}
