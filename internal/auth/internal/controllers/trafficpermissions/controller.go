package trafficpermissions

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Mapper is used to map a watch event for a TrafficPermission resource and translate
// it to a ComputedTrafficPermissions resource which contains the effective permissions
// from the TrafficPermission resource.
type Mapper interface {
	// MapWorkloadIdentity will take a WorkloadIdentity resource and return controller requests for all
	// ComputedTrafficPermissions associated with that workload.
	MapWorkloadIdentity(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// MapTrafficPermission will take a TrafficPermission resource and return controller requests for all
	// ComputedTrafficPermissions associated with that workload.
	MapTrafficPermission(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// UntrackComputedTrafficPermission instructs the Mapper to forget about any
	// association it was tracking for this ComputedTrafficPermission.
	UntrackComputedTrafficPermission(*pbresource.ID)

	WorkloadIdentityFromCTP(*pbresource.Resource, *pbauth.ComputedTrafficPermission) *pbresource.ID
}

// Controller creates a controller for automatic ComputedTrafficPermissions management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller(mapper Mapper) controller.Controller {
	if mapper == nil {
		panic("No WorkloadMapper was provided to the ServiceEndpointsController constructor")
	}

	return controller.ForType(types.ComputedTrafficPermissionType).
		WithWatch(types.TrafficPermissionType, mapper.MapTrafficPermission).
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

	// TODO: Need to add a UUID to new ComputedTrafficPermissions or some other unique name
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		// What would possibly cause this? We won't allow direct crud ops
		// on CTP, so I don't think that this should really happen.
		rt.Logger.Trace("")
		r.mapper.UntrackComputedTrafficPermission(req.ID)
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}

	res := rsp.Resource
	var ctp pbauth.ComputedTrafficPermission
	if err := res.Data.UnmarshalTo(&ctp); err != nil {
		rt.Logger.Error("error unmarshalling computed traffic permission data", "error", err)
		return err
	}

	// Check the workload identity associated to the CTP
	workloadIdentityID := r.mapper.WorkloadIdentityFromCTP(res, &ctp)
	wi, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: workloadIdentityID})
	switch {
	case status.Code(err) == codes.NotFound:
		// the WI has been deleted. Remove the CTP (? or should we leave it)
		// and untrack the WI
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}

	return nil
}
