// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import "github.com/hashicorp/consul/proto-public/pbresource"

const (
	StatusKey                       = "consul.io/endpoint-manager"
	StatusConditionEndpointsManaged = "EndpointsManaged"

	StatusReasonSelectorNotFound = "SelectorNotFound"
	StatusReasonSelectorFound    = "SelectorFound"

	SelectorFoundMessage    = "A valid workload selector is present within the service."
	SelectorNotFoundMessage = "Either the workload selector was not present or contained no selection criteria."
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
)
