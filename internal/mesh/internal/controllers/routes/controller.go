// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/xroutemapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	metaManagedBy = "managed-by-controller"
)

func Controller() controller.Controller {
	mapper := xroutemapper.New()

	r := &routesReconciler{
		mapper: mapper,
	}
	return controller.ForType(types.ComputedRoutesType).
		WithWatch(types.HTTPRouteType, mapper.MapHTTPRoute).
		WithWatch(types.GRPCRouteType, mapper.MapGRPCRoute).
		WithWatch(types.TCPRouteType, mapper.MapTCPRoute).
		WithWatch(types.DestinationPolicyType, mapper.MapDestinationPolicy).
		WithWatch(catalog.FailoverPolicyType, mapper.MapFailoverPolicy).
		WithWatch(catalog.ServiceType, mapper.MapService).
		WithReconciler(r)
}

type routesReconciler struct {
	mapper *xroutemapper.Mapper
}

func (r *routesReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// Notably don't inject the resource-id here, since we have to do a fan-out
	// to multiple resources due to xRoutes having multiple parent refs.
	rt.Logger = rt.Logger.With("controller", StatusKey)
	// rt.Logger = rt.Logger.With("event-resource-id", resource.IDToString(req.ID))

	rt.Logger.Trace("reconciling computed routes")

	loggerFor := func(id *pbresource.ID) hclog.Logger {
		return rt.Logger.With("resource-id", resource.IDToString(id))
	}
	related, err := loader.LoadResourcesForComputedRoutes(ctx, loggerFor, rt.Client, r.mapper, req.ID)
	if err != nil {
		rt.Logger.Error("error loading relevant resources", "error", err)
		return err
	}

	pending := make(PendingStatuses)

	ValidateXRouteReferences(related, pending)

	generatedResults := GenerateComputedRoutes(ctx, rt.Logger, related, pending)

	if err := UpdatePendingStatuses(ctx, rt, pending); err != nil {
		rt.Logger.Error("error updating statuses for affected relevant resources", "error", err)
		return err
	}

	for _, result := range generatedResults {
		computedRoutesID := result.ID

		logger := rt.Logger.With("resource-id", resource.IDToString(computedRoutesID))

		prev, err := resource.GetDecodedResource[pbmesh.ComputedRoutes, *pbmesh.ComputedRoutes](ctx, rt.Client, computedRoutesID)
		if err != nil {
			logger.Error("error loading previous computed routes", "error", err)
			return err
		}

		if err := ensureComputedRoutesIsSynced(ctx, logger, rt.Client, result, prev); err != nil {
			return err
		}
	}

	return nil
}

func ensureComputedRoutesIsSynced(
	ctx context.Context,
	logger hclog.Logger,
	client pbresource.ResourceServiceClient,
	result *ComputedRoutesResult,
	prev *types.DecodedComputedRoutes,
) error {
	if result.Data == nil {
		logger.Info("DELETE WRITE")
		return deleteComputedRoutes(ctx, logger, client, prev)
	}

	// Upsert the resource if changed.
	if prev != nil {
		if proto.Equal(prev.Data, result.Data) {
			logger.Info("SKIPPING WRITE")
			return nil // no change
		}
		result.ID = prev.Resource.Id
	}

	logger.Info("UPSERT WRITE")
	return upsertComputedRoutes(ctx, logger, client, result.ID, result.OwnerID, result.Data)
}

func upsertComputedRoutes(
	ctx context.Context,
	logger hclog.Logger,
	client pbresource.ResourceServiceClient,
	id *pbresource.ID,
	ownerID *pbresource.ID,
	data *pbmesh.ComputedRoutes,
) error {
	mcData, err := anypb.New(data)
	if err != nil {
		logger.Error("error marshalling new computed routes payload", "error", err)
		return err
	}

	// Now perform the write. The computed routes resource should be owned
	// by the service so that it will automatically be deleted upon service
	// deletion.

	_, err = client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:    id,
			Owner: ownerID,
			Metadata: map[string]string{
				metaManagedBy: StatusKey,
			},
			Data: mcData,
		},
	})
	if err != nil {
		logger.Error("error writing generated mesh config", "error", err)
		return err
	}

	logger.Trace("updated mesh config was successfully written")

	return nil
}

func deleteComputedRoutes(
	ctx context.Context,
	logger hclog.Logger,
	client pbresource.ResourceServiceClient,
	prev *types.DecodedComputedRoutes,
) error {
	if prev == nil {
		return nil
	}

	// The service the mesh config controls no longer participates in the
	// mesh at all.

	logger.Trace("removing previous computed routes")

	// This performs a CAS deletion.
	_, err := client.Delete(ctx, &pbresource.DeleteRequest{
		Id:      prev.Resource.Id,
		Version: prev.Resource.Version,
	})
	// Potentially we could look for CAS failures by checking if the gRPC
	// status code is Aborted. However its an edge case and there could
	// possibly be other reasons why the gRPC status code would be aborted
	// besides CAS version mismatches. The simplest thing to do is to just
	// propagate the error and retry reconciliation later.
	if err != nil {
		logger.Error("error deleting previous mesh config", "error", err)
		return err
	}

	return nil
}
