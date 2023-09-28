// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey                       = "consul.io/endpoint-manager"
	StatusConditionEndpointsManaged = "EndpointsManaged"

	StatusReasonSelectorNotFound = "SelectorNotFound"
	StatusReasonSelectorFound    = "SelectorFound"

	SelectorFoundMessage    = "A valid workload selector is present within the service."
	SelectorNotFoundMessage = "Either the workload selector was not present or contained no selection criteria."

	StatusConditionBoundIdentities = "BoundIdentities"

	StatusReasonWorkloadIdentitiesFound   = "WorkloadIdentitiesFound"
	StatusReasonNoWorkloadIdentitiesFound = "NoWorkloadIdentitiesFound"

	IdentitiesFoundMessageFormat     = "Found workload identities associated with this service: %q."
	IdentitiesNotFoundChangedMessage = "No associated workload identities found."
)

var (
	ConditionManaged = &pbresource.Condition{
		Type:    StatusConditionEndpointsManaged,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonSelectorFound,
		Message: SelectorFoundMessage,
	}

	ConditionUnmanaged = &pbresource.Condition{
		Type:    StatusConditionEndpointsManaged,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonSelectorNotFound,
		Message: SelectorNotFoundMessage,
	}

	ConditionIdentitiesNotFound = &pbresource.Condition{
		Type:    StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonNoWorkloadIdentitiesFound,
		Message: IdentitiesNotFoundChangedMessage,
	}
)

func ConditionIdentitiesFound(identities []string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonWorkloadIdentitiesFound,
		Message: fmt.Sprintf(IdentitiesFoundMessageFormat, strings.Join(identities, ",")),
	}
}
