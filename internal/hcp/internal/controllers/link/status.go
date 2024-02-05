// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package link

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey = "consul.io/hcp/link"

	// Statuses
	StatusLinked    = "linked"
	StatusValidated = "validated"

	LinkedSuccessReason                              = "SUCCESS"
	LinkedFailedReason                               = "FAILED"
	LinkedDisabledReasonV2ResourcesUnsupportedReason = "DISABLED_V2_RESOURCES_UNSUPPORTED"
	LinkedUnauthorizedReason                         = "UNAUTHORIZED"
	LinkedForbiddenReason                            = "FORBIDDEN"
	ValidatedSuccessReason                           = "SUCCESS"
	ValidatedFailedV2ResourcesReason                 = "V2_RESOURCES_UNSUPPORTED"

	LinkedMessageFormat                = "Successfully linked to cluster '%s'"
	FailedMessage                      = "Failed to link to HCP due to unexpected error"
	DisabledResourceAPIsEnabledMessage = "Link is disabled because resource-apis are enabled"
	UnauthorizedMessage                = "Access denied, check client_id and client_secret"
	ForbiddenMessage                   = "Access denied, check the resource_id"
	ValidatedSuccessMessage            = "Successfully validated link"
	ValidatedFailedV2ResourcesMessage  = "Link is disabled because resource-apis are enabled"
)

var (
	ConditionDisabled = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  LinkedDisabledReasonV2ResourcesUnsupportedReason,
		Message: DisabledResourceAPIsEnabledMessage,
	}
	ConditionFailed = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  LinkedFailedReason,
		Message: FailedMessage,
	}
	ConditionUnauthorized = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  LinkedUnauthorizedReason,
		Message: UnauthorizedMessage,
	}
	ConditionForbidden = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  LinkedForbiddenReason,
		Message: ForbiddenMessage,
	}
	ConditionValidatedSuccess = &pbresource.Condition{
		Type:    StatusValidated,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  ValidatedSuccessReason,
		Message: ValidatedSuccessMessage,
	}
	ConditionValidatedFailed = &pbresource.Condition{
		Type:    StatusValidated,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  ValidatedFailedV2ResourcesReason,
		Message: ValidatedFailedV2ResourcesMessage,
	}
)

func ConditionLinked(resourceId string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  LinkedSuccessReason,
		Message: fmt.Sprintf(LinkedMessageFormat, resourceId),
	}
}

func writeStatusIfNotEqual(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, status *pbresource.Status) error {
	if resource.EqualStatus(res.Status[StatusKey], status, false) {
		return nil
	}
	_, err := rt.Client.WriteStatus(
		ctx, &pbresource.WriteStatusRequest{
			Id:     res.Id,
			Key:    StatusKey,
			Status: status,
		},
	)
	if err != nil {
		rt.Logger.Error("error writing link status", "error", err)
	}
	return err
}

func linkingFailedCondition(err error) *pbresource.Condition {
	switch {
	case errors.Is(err, client.ErrUnauthorized):
		return ConditionUnauthorized
	case errors.Is(err, client.ErrForbidden):
		return ConditionForbidden
	default:
		return ConditionFailed
	}
}

func IsLinked(res *pbresource.Resource) (linked bool, reason string) {
	return isConditionTrue(res, StatusLinked)
}

func IsValidated(res *pbresource.Resource) (linked bool, reason string) {
	return isConditionTrue(res, StatusValidated)
}

func isConditionTrue(res *pbresource.Resource, statusType string) (bool, string) {
	if !resource.EqualType(res.GetId().GetType(), pbhcp.LinkType) {
		return false, "resource is not hcp.Link type"
	}

	linkStatus, ok := res.GetStatus()[StatusKey]
	if !ok {
		return false, "link status not set"
	}

	for _, cond := range linkStatus.GetConditions() {
		if cond.Type == statusType && cond.GetState() == pbresource.Condition_STATE_TRUE {
			return true, ""
		}
	}
	return false, fmt.Sprintf("link status does not include positive %s condition", statusType)
}
