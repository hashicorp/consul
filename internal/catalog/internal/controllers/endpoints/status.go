// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"sort"
	"strings"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ControllerID                    = "consul.io/endpoint-manager"
	StatusConditionEndpointsManaged = "EndpointsManaged"

	StatusReasonSelectorNotFound = "SelectorNotFound"
	StatusReasonSelectorFound    = "SelectorFound"

	selectorFoundMessage    = "A valid workload selector is present within the service."
	selectorNotFoundMessage = "Either the workload selector was not present or contained no selection criteria."

	StatusConditionBoundIdentities = "BoundIdentities"

	StatusReasonWorkloadIdentitiesFound   = "WorkloadIdentitiesFound"
	StatusReasonNoWorkloadIdentitiesFound = "NoWorkloadIdentitiesFound"
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
		Message: "",
	}
)

func ConditionIdentitiesFound(identities []string) *pbresource.Condition {
	sort.Strings(identities)

	return &pbresource.Condition{
		Type:    StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonWorkloadIdentitiesFound,
		Message: strings.Join(identities, ","),
	}
}
