// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package explicitdestinations

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/workloadselectionmapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const ControllerName = "consul.io/explicit-destinations-controller"

func Controller(destinationsMapper *workloadselectionmapper.Mapper[*pbmesh.Destinations]) controller.Controller {
	if destinationsMapper == nil {
		panic("destinations mapper is required")
	}

	return controller.ForType(pbmesh.ComputedDestinationsType).
		WithWatch(pbmesh.DestinationsType, destinationsMapper.MapToComputedType).
		WithWatch(pbcatalog.WorkloadType, controller.ReplaceType(pbmesh.ComputedDestinationsType)).
		WithReconciler(&reconciler{destinations: destinationsMapper})
}

type reconciler struct {
	destinations *workloadselectionmapper.Mapper[*pbmesh.Destinations]
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("controller", ControllerName, "id", req.ID)

	// Look up the associated workload.
	workloadID := resource.ReplaceType(pbcatalog.WorkloadType, req.ID)
	workload, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, rt.Client, workloadID)
	if err != nil {
		rt.Logger.Error("error fetching workload", "error", err)
		return err
	}

	// If workload is not found, the decoded resource will be nil.
	if workload == nil || workload.GetResource() == nil || workload.GetData() == nil {
		// When workload is not there, we don't need to manually delete the resource
		// because it is owned by the workload. In this case, we skip reconcile
		// because there's nothing for us to do.
		rt.Logger.Trace("the corresponding workload does not exist", "id", workloadID)
		return nil
	}

	// Get existing ComputedDestinations resource (if any).
	cpc, err := resource.GetDecodedResource[*pbmesh.ComputedDestinations](ctx, rt.Client, req.ID)
	if err != nil {
		rt.Logger.Error("error fetching ComputedDestinations", "error", err)
		return err
	}

	// If workload is not on the mesh, we need to delete the resource and return
	// as for non-mesh workloads there should be no destinations.
	if !workload.GetData().IsMeshEnabled() {
		rt.Logger.Trace("workload is not on the mesh, skipping reconcile and deleting any corresponding ComputedDestinations", "id", workloadID)

		// Delete CD only if it exists.
		if cpc != nil {
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				// If there's an error deleting CD, we want to re-trigger reconcile again.
				rt.Logger.Error("error deleting ComputedDestinations", "error", err)
				return err
			}
		}

		// Otherwise, return as there's nothing else for us to do.
		return nil
	}

	// Now get any destinations that we have in the cache that have selectors matching the name
	// of this CD (name-aligned with workload).
	destinationIDs := r.destinations.IDsForWorkload(req.ID.GetName())

	decodedDestinations, err := r.fetchDestinations(ctx, rt.Client, destinationIDs)
	if err != nil {
		rt.Logger.Error("error fetching destinations", "error", err)
		return err
	}

	newComputedDestinationsData := &pbmesh.ComputedDestinations{}
	for _, dst := range decodedDestinations {
		valid, cond := validate(ctx, rt.Client, dst)

		// Only add it to computed destinations if its destinations are valid.
		if valid {
			newComputedDestinationsData.Destinations = append(newComputedDestinationsData.Destinations, dst.GetData().GetDestinations()...)
		}

		// Write status for this destination.
		currentStatus := dst.GetResource().GetStatus()[ControllerName]
		updatedStatus := &pbresource.Status{
			Conditions:         []*pbresource.Condition{cond},
			ObservedGeneration: dst.GetResource().GetGeneration(),
		}

		// If the status is unchanged then we should return and avoid the unnecessary write
		if !resource.EqualStatus(currentStatus, updatedStatus, false) {
			rt.Logger.Trace("updating status", "id", dst.GetResource().GetId())
			_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
				Id:     dst.GetResource().GetId(),
				Key:    ControllerName,
				Status: updatedStatus,
			})
			if err != nil {
				rt.Logger.Error("error writing new status", "id", dst.GetResource().GetId(), "error", err)
				return err
			}
		}
	}

	// If after fetching and validating, we don't have any destinations,
	// we need to skip reconcile and delete the resource.
	if len(newComputedDestinationsData.GetDestinations()) == 0 {
		rt.Logger.Trace("found no destinations associated with this workload")

		if cpc != nil {
			rt.Logger.Trace("deleting ComputedDestinations")
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				// If there's an error deleting CD, we want to re-trigger reconcile again.
				rt.Logger.Error("error deleting ComputedDestinations", "error", err)
				return err
			}
		}

		return nil
	}

	// Lastly, write the resource.
	if cpc == nil || !proto.Equal(cpc.GetData(), newComputedDestinationsData) {
		rt.Logger.Trace("writing new ComputedDestinations")

		// First encode the endpoints data as an Any type.
		cpcDataAsAny, err := anypb.New(newComputedDestinationsData)
		if err != nil {
			rt.Logger.Error("error marshalling latest ComputedDestinations", "error", err)
			return err
		}

		_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
			Resource: &pbresource.Resource{
				Id:    req.ID,
				Owner: workloadID,
				Data:  cpcDataAsAny,
			},
		})
		if err != nil {
			rt.Logger.Error("error writing ComputedDestinations", "error", err)
			return err
		}
	}

	return nil
}

func validate(
	ctx context.Context,
	client pbresource.ResourceServiceClient,
	destinations *types.DecodedDestinations) (bool, *pbresource.Condition) {
	for _, dest := range destinations.GetData().GetDestinations() {
		serviceRef := resource.ReferenceToString(dest.DestinationRef)

		// Fetch and validate service.
		service, err := resource.GetDecodedResource[*pbcatalog.Service](ctx, client, resource.IDFromReference(dest.DestinationRef))
		if err != nil {
			return false, ConditionDestinationServiceReadError(serviceRef)
		}
		if service == nil {
			return false, ConditionDestinationServiceNotFound(serviceRef)
		}

		if !service.GetData().IsMeshEnabled() {
			return false, ConditionMeshProtocolNotFound(serviceRef)
		}

		if service.GetData().FindServicePort(dest.DestinationPort).Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			return false, ConditionMeshProtocolDestinationPort(serviceRef, dest.DestinationPort)
		}

		// Fetch and validate computed routes for service.
		serviceID := resource.IDFromReference(dest.DestinationRef)
		cr, err := resource.GetDecodedResource[*pbmesh.ComputedRoutes](ctx, client, resource.ReplaceType(pbmesh.ComputedRoutesType, serviceID))
		if err != nil {
			return false, ConditionDestinationComputedRoutesReadErr(serviceRef)
		}
		if cr == nil {
			return false, ConditionDestinationComputedRoutesNotFound(serviceRef)
		}

		_, ok := cr.Data.PortedConfigs[dest.DestinationPort]
		if !ok {
			return false, ConditionDestinationComputedRoutesPortNotFound(serviceRef, dest.DestinationPort)
		}

		// Otherwise, continue to the next destination.
	}

	return true, ConditionDestinationsAccepted()
}

func (r *reconciler) fetchDestinations(
	ctx context.Context,
	client pbresource.ResourceServiceClient,
	destinationIDs []*pbresource.ID) ([]*types.DecodedDestinations, error) {

	var decoded []*types.DecodedDestinations
	for _, id := range destinationIDs {
		res, err := resource.GetDecodedResource[*pbmesh.Destinations](ctx, client, id)
		if err != nil {
			return nil, err
		}
		if res == nil || res.GetResource() == nil || res.GetData() == nil {
			// If resource is not found, we should untrack it.
			r.destinations.UntrackID(id)
			continue
		}
		decoded = append(decoded, res)
	}

	return decoded, nil
}
