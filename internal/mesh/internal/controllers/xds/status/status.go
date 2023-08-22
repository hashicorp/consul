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
