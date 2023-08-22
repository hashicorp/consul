package trafficpermissions

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	// MapWorkloadIdentity will take a WorkloadIdentity resource and return controller requests for all
	// ComputedTrafficPermissions associated with that workload.
	MapWorkloadIdentity(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// MapTrafficPermission will take a TrafficPermission resource and return controller requests for all
	// ComputedTrafficPermissions associated with that workload.
	MapTrafficPermission(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// UntrackComputedTrafficPermission instructs the Mapper to forget about any
	// association it was tracking for this ComputedTrafficPermission.
	UntrackComputedTrafficPermission(computedTrafficPermissionID *pbresource.ID)
}

// Controller creates a controller for automatic ComputedTrafficPermissions management for
// updates to WorkloadIdentity or TrafficPermission resources.
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
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)

	rt.Logger.Trace("reconciling computed traffic permissions")

	// Read ComputedTrafficPermission by ID
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		rt.Logger.Trace("computed traffic permission has been deleted")
		r.mapper.UntrackComputedTrafficPermission(req.ID)
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}

	res := rsp.Resource
	var ctp pbcatalog.ComputedTrafficPermission

	return nil
}
