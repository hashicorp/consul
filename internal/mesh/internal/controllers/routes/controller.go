// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ControllerID is the name for this controller. It's used for logging or status keys.
const ControllerID = "consul.io/routes-controller"

func Controller() *controller.Controller {
	boundRefsMapper := dependency.CacheListMapper(pbmesh.ComputedRoutesType, boundRefsIndex.Name())

	return controller.NewController(ControllerID,
		pbmesh.ComputedRoutesType,
		boundRefsIndex,
	).
		WithWatch(pbmesh.HTTPRouteType,
			// BoundRefs: the ParentRef slice is an interior up-pointer and may change.
			dependency.MultiMapper(boundRefsMapper, MapHTTPRoute),
			httpRouteByParentServiceIndex,
			httpRouteByBackendServiceIndex,
		).
		WithWatch(pbmesh.GRPCRouteType,
			// BoundRefs: the ParentRef slice is an interior up-pointer and may change.
			dependency.MultiMapper(boundRefsMapper, MapGRPCRoute),
			grpcRouteByParentServiceIndex,
			grpcRouteByBackendServiceIndex,
		).
		WithWatch(pbmesh.TCPRouteType,
			// BoundRefs: the ParentRef slice is an interior up-pointer and may change.
			dependency.MultiMapper(boundRefsMapper, MapTCPRoute),
			tcpRouteByParentServiceIndex,
			tcpRouteByBackendServiceIndex,
		).
		WithWatch(pbmesh.DestinationPolicyType,
			// BoundRefs: none
			MapServiceNameAligned,
		).
		WithWatch(pbcatalog.ComputedFailoverPolicyType,
			// BoundRefs: none
			MapServiceNameAligned,
			computedFailoverByDestRefIndex,
		).
		WithWatch(pbcatalog.ServiceType,
			// BoundRefs: none
			MapService,
		).
		WithReconciler(&routesReconciler{})
}

type routesReconciler struct {
}

func (r *routesReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// Notably don't inject this as "resource-id" here into the logger, since
	// we have to do a fan-out to multiple resources due to xRoutes having
	// multiple parent refs.
	rt.Logger = rt.Logger.With("seed-resource-id", req.ID, "controller", ControllerID)

	rt.Logger.Trace("reconciling computed routes")

	related, err := LoadResourcesForComputedRoutes(rt.Cache, req.ID)
	if err != nil {
		rt.Logger.Error("error loading relevant resources", "error", err)
		return err
	}

	pending := make(PendingStatuses)

	ValidateXRouteReferences(related, pending)
	ValidateDestinationPolicyPorts(related, pending)

	generatedResults := GenerateComputedRoutes(related, pending)

	if err := UpdatePendingStatuses(ctx, rt, pending); err != nil {
		rt.Logger.Error("error updating statuses for affected relevant resources", "error", err)
		return err
	}

	for _, result := range generatedResults {
		computedRoutesID := result.ID

		logger := rt.Logger.With("resource-id", computedRoutesID)

		prev, err := cache.GetDecoded[*pbmesh.ComputedRoutes](rt.Cache, pbmesh.ComputedRoutesType, "id", computedRoutesID)
		if err != nil {
			logger.Error("error loading previous computed routes", "error", err)
			return err
		}

		if err := r.ensureComputedRoutesIsSynced(ctx, rt, result, prev); err != nil {
			return err
		}
	}

	return nil
}

func (r *routesReconciler) ensureComputedRoutesIsSynced(
	ctx context.Context,
	rt controller.Runtime,
	result *ComputedRoutesResult,
	prev *types.DecodedComputedRoutes,
) error {
	if result.Data == nil {
		return r.deleteComputedRoutes(ctx, rt, prev)
	}

	// Upsert the resource if changed.
	if prev != nil {
		if proto.Equal(prev.Data, result.Data) {
			return nil // no change
		}
		result.ID = prev.Resource.Id
	}

	mcData, err := anypb.New(result.Data)
	if err != nil {
		rt.Logger.Error("error marshalling new computed routes payload", "error", err)
		return err
	}

	next := &pbresource.Resource{
		Id:    result.ID,
		Owner: result.OwnerID,
		Data:  mcData,
	}

	return r.upsertComputedRoutes(ctx, rt, next)
}

func (r *routesReconciler) upsertComputedRoutes(
	ctx context.Context,
	rt controller.Runtime,
	res *pbresource.Resource,
) error {
	// Now perform the write. The computed routes resource should be owned
	// by the service so that it will automatically be deleted upon service
	// deletion.

	_, err := rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: res,
	})
	if err != nil {
		rt.Logger.Error("error writing computed routes", "error", err)
		return err
	}

	rt.Logger.Trace("updated computed routes resource was successfully written")

	return nil
}

func (r *routesReconciler) deleteComputedRoutes(ctx context.Context, rt controller.Runtime, prev *types.DecodedComputedRoutes) error {
	if prev == nil {
		return nil
	}

	// The service the computed routes controls no longer participates in the
	// mesh at all.

	rt.Logger.Trace("removing previous computed routes")

	// This performs a CAS deletion.
	_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{
		Id:      prev.Resource.Id,
		Version: prev.Resource.Version,
	})
	// Potentially we could look for CAS failures by checking if the gRPC
	// status code is Aborted. However its an edge case and there could
	// possibly be other reasons why the gRPC status code would be aborted
	// besides CAS version mismatches. The simplest thing to do is to just
	// propagate the error and retry reconciliation later.
	if err != nil {
		rt.Logger.Error("error deleting previous computed routes resource", "error", err)
		return err
	}

	return nil
}
