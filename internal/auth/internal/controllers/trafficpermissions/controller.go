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

	/*
	 * A CTP ID could come in for a variety or reasons.
	 * 1. workload identity create / delete
	 * 2. traffic permission create / delete
	 *
	 * We need to take the CTP ID and map it back to the relevant
	 * workloads and traffic permissions.
	 *
	 * Mappings must be maintained for
	 * 1. workloadIdentity -> computedTrafficPermission (CTP which represents that WI as a destination)
	 * 2. workloadIdentity -> []trafficPermission (TP which affect the CTP for the WI)
	 * 3. trafficPermissions -> []workloadIdentity (WI which are affected by the TP)
	 * 4. trafficPermissions -> []computedTrafficPermission (CTP affected by the TP, only one if explicit dest)
	 * 5. computedTrafficPermission -> workloadIdentity (WI which is the destination for the CTP)
	 * 6. computedTrafficPermission -> []trafficPermission (TP which affect the CTP)
	 *
	 * First, look up the workload identity that maps to the CTP.
	 * If not found, the WI has been deleted. Untrack the WI and delete the CTP.
	 * Else, it must have been a traffic permissions update.
	 * TODO: Except if it is a new WI. Where do we create a new CTP? We can't just ship some non-existent ID so mapper?
	 *
	 * Use the WI to grab all the traffic permissions that apply to that WI. We will likely need to store the
	 * TP as a radix tree based on their destinations. Then we can find all the matching TP to the WI.
	 *
	 * Take the list of TP IDs and then look up each one. If its missing, then it was deleted so untrack it.
	 * Recompute the CTP from the list of TPs. I originally though we would have to recompute all of the CTPs which
	 * were affected by the deleted TP but I think that the mapper will sort of resolve this naturally. When
	 * a TP is deleted, the mapper will already add all of the affected CTPs to the reconcile queue so we don't have to
	 * do the reverse mapping and recalculate all in the reconcile.
	 */

	return nil
}
