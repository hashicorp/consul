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
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
)

const (
	StatusKey = "consul.io/hcp/link"

	StatusLinked                         = "linked"
	LinkedReason                         = "SUCCESS"
	FailedReason                         = "FAILED"
	DisabledReasonV2ResourcesUnsupported = "DISABLED_V2_RESOURCES_UNSUPPORTED"
	UnauthorizedReason                   = "UNAUTHORIZED"
	ForbiddenReason                      = "FORBIDDEN"

	LinkedMessageFormat                = "Successfully linked to cluster '%s'"
	FailedMessage                      = "Failed to link to HCP due to unexpected error"
	DisabledResourceAPIsEnabledMessage = "Link is disabled because resource-apis are enabled"
	UnauthorizedMessage                = "Access denied, check client_id and client_secret"
	ForbiddenMessage                   = "Access denied, check the resource_id"
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
	ConditionUnauthorized = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  UnauthorizedReason,
		Message: UnauthorizedMessage,
	}
	ConditionForbidden = &pbresource.Condition{
		Type:    StatusLinked,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  ForbiddenReason,
		Message: ForbiddenMessage,
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

func writeStatusIfNotEqual(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, status *pbresource.Status) error {
	if resource.EqualStatus(res.Status[StatusKey], status, false) {
		return nil
	}
	_, err := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    StatusKey,
		Status: status,
	})
	return err
}

func linkingFailed(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, err error) error {
	var condition *pbresource.Condition
	switch {
	case errors.Is(err, client.ErrUnauthorized):
		condition = ConditionUnauthorized
	case errors.Is(err, client.ErrForbidden):
		condition = ConditionForbidden
	default:
		condition = ConditionFailed
	}
	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions:         []*pbresource.Condition{condition},
	}

	writeErr := writeStatusIfNotEqual(ctx, rt, res, newStatus)
	if writeErr != nil {
		rt.Logger.Error("error writing status", "error", writeErr)
		return writeErr
	}

	return nil
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
