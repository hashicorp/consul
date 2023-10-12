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

	selectorFoundMessage    = "A valid workload selector is present within the service."
	selectorNotFoundMessage = "Either the workload selector was not present or contained no selection criteria."

	StatusConditionBoundIdentities = "BoundIdentities"

	StatusReasonWorkloadIdentitiesFound   = "WorkloadIdentitiesFound"
	StatusReasonNoWorkloadIdentitiesFound = "NoWorkloadIdentitiesFound"

	identitiesFoundMessageFormat     = "Found workload identities associated with this service: %q."
	identitiesNotFoundChangedMessage = "No associated workload identities found."
)

var (
	ConditionManaged = &pbresource.Condition{
		Type:    StatusConditionEndpointsManaged,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonSelectorFound,
		Message: selectorFoundMessage,
	}

	ConditionUnmanaged = &pbresource.Condition{
		Type:    StatusConditionEndpointsManaged,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonSelectorNotFound,
		Message: selectorNotFoundMessage,
	}

	ConditionIdentitiesNotFound = &pbresource.Condition{
		Type:    StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonNoWorkloadIdentitiesFound,
		Message: identitiesNotFoundChangedMessage,
	}
)

func ConditionIdentitiesFound(identities []string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonWorkloadIdentitiesFound,
		Message: fmt.Sprintf(identitiesFoundMessageFormat, strings.Join(identities, ",")),
	}
}
