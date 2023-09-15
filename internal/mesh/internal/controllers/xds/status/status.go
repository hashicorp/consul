// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package status

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusConditionProxyStateAccepted             = "ProxyStateAccepted"
	StatusReasonNilProxyState                     = "ProxyStateNil"
	StatusReasonProxyStateReferencesComputed      = "ProxyStateReferencesComputed"
	StatusReasonEndpointNotRead                   = "ProxyStateEndpointReferenceReadError"
	StatusReasonCreatingProxyStateEndpointsFailed = "ProxyStateEndpointsNotComputed"
	StatusReasonPushChangeFailed                  = "ProxyStatePushChangeFailed"
	StatusReasonTrustBundleFetchFailed            = "ProxyStateTrustBundleFetchFailed"
	StatusReasonLeafWatchSetupFailed              = "ProxyStateLeafWatchSetupError"
	StatusReasonLeafFetchFailed                   = "ProxyStateLeafFetchError"
	StatusReasonLeafEmpty                         = "ProxyStateLeafEmptyError"
)

func KeyFromID(id *pbresource.ID) string {
	return fmt.Sprintf("%s/%s/%s",
		resource.ToGVK(id.Type),
		resource.TenancyToString(id.Tenancy),
		id.Name)
}

func ConditionAccepted() *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  StatusReasonProxyStateReferencesComputed,
		Message: fmt.Sprintf("proxy state was computed and pushed."),
	}
}
func ConditionRejectedNilProxyState(pstRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonNilProxyState,
		Message: fmt.Sprintf("nil proxy state is not valid %q.", pstRef),
	}
}
func ConditionRejectedErrorCreatingLeafWatch(leafRef string, err string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonLeafWatchSetupFailed,
		Message: fmt.Sprintf("error creating leaf watch %q: %s", leafRef, err),
	}
}
func ConditionRejectedErrorGettingLeaf(leafRef string, err string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonLeafFetchFailed,
		Message: fmt.Sprintf("error getting leaf from leaf certificate manager %q: %s", leafRef, err),
	}
}
func ConditionRejectedErrorCreatingProxyStateLeaf(leafRef string, err string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonLeafEmpty,
		Message: fmt.Sprintf("error getting leaf certificate contents %q: %s", leafRef, err),
	}
}
func ConditionRejectedErrorReadingEndpoints(endpointRef string, err string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonEndpointNotRead,
		Message: fmt.Sprintf("error reading referenced service endpoints %q: %s", endpointRef, err),
	}
}
func ConditionRejectedCreatingProxyStateEndpoints(endpointRef string, err string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonCreatingProxyStateEndpointsFailed,
		Message: fmt.Sprintf("could not create proxy state endpoints from service endpoints %q: %s", endpointRef, err),
	}
}
func ConditionRejectedPushChangeFailed(pstRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonPushChangeFailed,
		Message: fmt.Sprintf("failed to push change for proxy state template %q", pstRef),
	}
}
func ConditionRejectedTrustBundleFetchFailed(pstRef string) *pbresource.Condition {
	return &pbresource.Condition{
		Type:    StatusConditionProxyStateAccepted,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  StatusReasonTrustBundleFetchFailed,
		Message: fmt.Sprintf("failed to fetch trust bundle for proxy state template %q", pstRef),
	}
}

// WriteStatusIfChanged updates the ProxyStateTemplate status if it has changed.
func WriteStatusIfChanged(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, condition *pbresource.Condition) {
	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions: []*pbresource.Condition{
			condition,
		},
	}
	// If the status is unchanged then we should return and avoid the unnecessary write
	const controllerName = "consul.io/xds-controller"
	if resource.EqualStatus(res.Status[controllerName], newStatus, false) {
		return
	} else {
		_, err := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
			Id:     res.Id,
			Key:    controllerName,
			Status: newStatus,
		})

		if err != nil {
			rt.Logger.Error("error updating the proxy state template status", "error", err, "proxyStateTeamplate", res.Id)
		}
	}
}
