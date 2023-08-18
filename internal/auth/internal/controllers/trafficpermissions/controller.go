package trafficpermissions

import (
	"context"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Mapper is used to map a watch event for a TrafficPermission resource and translate
// it to a ComputedTrafficPermissions resource which contains the effective permissions
// from the TrafficPermission resource.
//
// TODO: What if the traffic permission is a wildcard destination? Then we need to return
// several IDs for ComputedTrafficPermissions rather than just a single one
type Mapper interface {
	MapWorkloadIdentity(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)
	MapTrafficPermissionToComputedTrafficPermissions()

	TrackTrafficPermission()
	UntrackTrafficPermission()
	ComputedTrafficPermissionIDFromTrafficID()
}

// Controller creates a controller for automatic traffic permissions management for workload identities.
func Controller(mapper Mapper) controller.Controller {
	if mapper == nil {
		panic("No WorkloadMapper was provided to the ServiceEndpointsController constructor")
	}

	return controller.ForType(types.ComputedTrafficPermissionType).
		WithWatch(types.TrafficPermissionType, controller.ReplaceType(types.ComputedTrafficPermissionType)).
		WithWatch(types.WorkloadIdentityType, mapper.MapWorkloadIdentity).
		WithReconciler(&reconciler{mapper: mapper})
}

type reconciler struct {
	mapper Mapper
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	return nil
}
