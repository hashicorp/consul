// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package explicitdestinations

import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/explicitdestinations/mapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const ControllerName = "consul.io/explicit-mapper-controller"

func Controller(mapper *mapper.Mapper) *controller.Controller {
	if mapper == nil {
		panic("mapper is required")
	}

	return controller.NewController(ControllerName, pbmesh.ComputedExplicitDestinationsType).
		WithWatch(pbmesh.DestinationsType, mapper.MapDestinations).
		WithWatch(pbcatalog.WorkloadType, dependency.ReplaceType(pbmesh.ComputedExplicitDestinationsType)).
		WithWatch(pbcatalog.ServiceType, mapper.MapService).
		WithWatch(pbmesh.ComputedRoutesType, mapper.MapComputedRoute).
		WithReconciler(&reconciler{mapper: mapper})
}

type reconciler struct {
	mapper *mapper.Mapper
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
		r.mapper.UntrackComputedExplicitDestinations(req.ID)
		return nil
	}

	// Get existing ComputedExplicitDestinations resource (if any).
	ced, err := resource.GetDecodedResource[*pbmesh.ComputedExplicitDestinations](ctx, rt.Client, req.ID)
	if err != nil {
		rt.Logger.Error("error fetching ComputedExplicitDestinations", "error", err)
		return err
	}

	// If workload is not on the mesh, we need to delete the resource and return
	// as for non-mesh workloads there should be no mapper.
	if !workload.GetData().IsMeshEnabled() {
		rt.Logger.Trace("workload is not on the mesh, skipping reconcile and deleting any corresponding ComputedDestinations", "id", workloadID)
		r.mapper.UntrackComputedExplicitDestinations(req.ID)

		// Delete CED only if it exists.
		if ced != nil {
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

	// Now get any mapper that we have in the cache that have selectors matching the name
	// of this CD (name-aligned with workload).
	destinationIDs := r.mapper.DestinationsForWorkload(req.ID)
	rt.Logger.Trace("cached destinations IDs", "ids", destinationIDs)

	decodedDestinations, err := r.fetchDestinations(ctx, rt.Client, destinationIDs, workload)
	if err != nil {
		rt.Logger.Error("error fetching mapper", "error", err)
		return err
	}

	if len(decodedDestinations) > 0 {
		r.mapper.TrackDestinations(req.ID, decodedDestinations)
	} else {
		r.mapper.UntrackComputedExplicitDestinations(req.ID)
	}

	conflicts := findConflicts(decodedDestinations)

	newComputedDestinationsData := &pbmesh.ComputedExplicitDestinations{}
	for _, dst := range decodedDestinations {
		updatedStatus := &pbresource.Status{
			ObservedGeneration: dst.GetResource().GetGeneration(),
		}

		// First check if this resource has a conflict. If it does, update status and don't include it in the computed resource.
		if _, ok := conflicts[resource.NewReferenceKey(dst.GetResource().GetId())]; ok {
			rt.Logger.Trace("skipping this Destinations resource because it has conflicts with others", "id", dst.GetResource().GetId())
			updatedStatus.Conditions = append(updatedStatus.Conditions, ConditionConflictFound(workload.GetResource().GetId()))
		} else {
			valid, cond := validate(ctx, rt.Client, dst)

			// Only add it to computed mapper if its mapper are valid.
			if valid {
				newComputedDestinationsData.Destinations = append(newComputedDestinationsData.Destinations, dst.GetData().GetDestinations()...)
			} else {
				rt.Logger.Trace("Destinations is not valid", "condition", cond)
			}

			updatedStatus.Conditions = append(updatedStatus.Conditions, ConditionConflictNotFound, cond)
		}

		// Write status for this destination.
		currentStatus := dst.GetResource().GetStatus()[ControllerName]

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

		if ced != nil {
			rt.Logger.Trace("deleting ComputedDestinations")
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				// If there's an error deleting CD, we want to re-trigger reconcile again.
				rt.Logger.Error("error deleting ComputedExplicitDestinations", "error", err)
				return err
			}
		}

		return nil
	}

	// Lastly, write the resource.
	if ced == nil || !proto.Equal(ced.GetData(), newComputedDestinationsData) {
		rt.Logger.Trace("writing new ComputedExplicitDestinations")

		// First encode the endpoints data as an Any type.
		cpcDataAsAny, err := anypb.New(newComputedDestinationsData)
		if err != nil {
			rt.Logger.Error("error marshalling latest ComputedExplicitDestinations", "error", err)
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
			rt.Logger.Error("error writing ComputedExplicitDestinations", "error", err)
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

		if service.GetData().FindServicePort(dest.DestinationPort) != nil &&
			service.GetData().FindServicePort(dest.DestinationPort).Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
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
	destinationIDs []*pbresource.ID,
	workload *types.DecodedWorkload,
) ([]*types.DecodedDestinations, error) {
	// Sort all configs alphabetically.
	sort.Slice(destinationIDs, func(i, j int) bool {
		return destinationIDs[i].GetName() < destinationIDs[j].GetName()
	})

	var decoded []*types.DecodedDestinations
	for _, id := range destinationIDs {
		res, err := resource.GetDecodedResource[*pbmesh.Destinations](ctx, client, id)
		if err != nil {
			return nil, err
		}
		if res == nil || res.GetResource() == nil || res.GetData() == nil {
			// If resource is not found, we should untrack it.
			r.mapper.UntrackDestinations(id)
			continue
		}

		if res.Data.Workloads.Filter != "" {
			match, err := resource.FilterMatchesResourceMetadata(workload.Resource, res.Data.Workloads.Filter)
			if err != nil {
				return nil, fmt.Errorf("error checking selector filters: %w", err)
			}
			if !match {
				continue
			}
		}

		decoded = append(decoded, res)
	}

	return decoded, nil
}

// Find conflicts finds any resources where listen addresses of the destinations are conflicting.
// It will record both resources as conflicting in the resulting map.
func findConflicts(destinations []*types.DecodedDestinations) map[resource.ReferenceKey]struct{} {
	addresses := make(map[string]*pbresource.ID)
	duplicates := make(map[resource.ReferenceKey]struct{})

	for _, decDestinations := range destinations {
		for _, dst := range decDestinations.GetData().GetDestinations() {
			var address string

			switch dst.ListenAddr.(type) {
			case *pbmesh.Destination_IpPort:
				listenAddr := dst.GetListenAddr().(*pbmesh.Destination_IpPort)
				address = fmt.Sprintf("%s:%d", listenAddr.IpPort.GetIp(), listenAddr.IpPort.GetPort())
			case *pbmesh.Destination_Unix:
				listenAddr := dst.GetListenAddr().(*pbmesh.Destination_Unix)
				address = listenAddr.Unix.GetPath()
			default:
				continue
			}

			if id, ok := addresses[address]; ok {
				// if there's already a listen address for one of the mapper, that means we've found a duplicate.
				duplicates[resource.NewReferenceKey(decDestinations.GetResource().GetId())] = struct{}{}

				// Also record the original resource as conflicting one.
				duplicates[resource.NewReferenceKey(id)] = struct{}{}

				// Don't evaluate the rest of mapper in this resource because this resource already has a duplicate.
				break
			} else {
				// Otherwise, record this address.
				addresses[address] = decDestinations.GetResource().GetId()
			}
		}
	}

	return duplicates
}
