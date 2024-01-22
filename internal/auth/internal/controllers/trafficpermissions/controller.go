// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	boundRefsIndexName = "bound-refs"
	ctpForTPIndexName  = "ctp"
)

func mapTrafficPermissionDestinationIdentity(_ context.Context, _ controller.Runtime, tp *resource.DecodedResource[*pbauth.TrafficPermissions]) ([]controller.Request, error) {
	ctp := getCTPIDFromTP(tp)
	if ctp == nil {
		return nil, nil
	}
	return []controller.Request{{ID: ctp}}, nil
}

func indexCTPForTP(tp *resource.DecodedResource[*pbauth.TrafficPermissions]) (bool, []byte, error) {
	ctp := getCTPIDFromTP(tp)
	if ctp == nil {
		return false, nil, nil
	}
	return true, index.IndexFromRefOrID(ctp), nil
}

func getCTPIDFromTP(tp *resource.DecodedResource[*pbauth.TrafficPermissions]) *pbresource.ID {
	if tp.Data == nil || tp.Data.GetDestination().GetIdentityName() == "" {
		return nil
	}

	return &pbresource.ID{
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp.GetId().GetTenancy(),
		Name:    tp.Data.Destination.IdentityName,
	}
}

// Controller creates a controller for automatic ComputedTrafficPermissions management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller() *controller.Controller {
	return controller.NewController(ControllerID, pbauth.ComputedTrafficPermissionsType, indexers.BoundRefsIndex[*pbauth.ComputedTrafficPermissions](boundRefsIndexName)).
		WithWatch(pbauth.WorkloadIdentityType, dependency.ReplaceType(pbauth.ComputedTrafficPermissionsType)).
		WithWatch(pbauth.TrafficPermissionsType,
			dependency.MultiMapper(
				// Re-reconcile all ComputedTrafficPermissions that were already computed using a previous
				// generation of this TrafficPermissions resource. This is needed to handle the case where
				// the TrafficPermissions destination is modified and should no longer affect the CTP
				// that previously reference it.
				dependency.CacheListMapper(pbauth.ComputedTrafficPermissionsType, boundRefsIndexName),
				// Also map to the Destination Identity to the name aligned ComputedTrafficPermissions
				dependency.MapDecoded[*pbauth.TrafficPermissions](mapTrafficPermissionDestinationIdentity),
			),
			// Index the TrafficPermissions destination identity to ComputedTrafficPermissions relationship.
			// This is used to efficiently find all TrafficPermissions that reference a given destination identity.
			indexers.DecodedSingleIndexer[*pbauth.TrafficPermissions](
				ctpForTPIndexName,
				index.ReferenceOrIDFromArgs,
				indexCTPForTP,
			),
		).
		WithReconciler(controller.ReconcileFunc(Reconcile))
}

// Reconcile will reconcile one ComputedTrafficPermission (CTP) in response to some event.
// Events include adding, modifying or deleting a WorkloadIdentity or TrafficPermission.
func Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID)
	/*
	 * A CTP ID could come in for a variety of reasons.
	 * 1. workload identity create / delete: this results in the creation / deletion of a new CTP
	 * 2. traffic permission create / modify / delete: this results in a potential modification of an existing CTP
	 *
	 * Part 1: Handle Workload Identity changes:
	 * Check if the workload identity exists. If it doesn't we can stop here.
	 * CTPs are always generated from WorkloadIdentities, therefore the WI resource must already exist.
	 * If it is missing, that means it was deleted.
	 */
	ctpID := req.ID
	wi := resource.ReplaceType(pbauth.WorkloadIdentityType, ctpID)
	workloadIdentity, err := cache.GetDecoded[*pbauth.WorkloadIdentity](rt.Cache, pbauth.WorkloadIdentityType, "id", wi)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding Workload Identity", "error", err)
		return err
	}
	if workloadIdentity == nil || workloadIdentity.Resource == nil {
		rt.Logger.Trace("workload identity has been deleted")
		return nil
	}

	// Check if CTP exists:
	ctp, err := cache.GetDecoded[*pbauth.ComputedTrafficPermissions](rt.Cache, pbauth.ComputedTrafficPermissionsType, "id", ctpID)
	if err != nil {
		rt.Logger.Error("error retrieving computed permissions", "error", err)
		return err
	}
	var oldResource *pbresource.Resource
	var owner *pbresource.ID
	if ctp == nil {
		// CTP does not yet exist, so we need to make a new one
		rt.Logger.Trace("creating new computed traffic permissions for workload identity")
		owner = workloadIdentity.Resource.Id
	} else {
		oldResource = ctp.Resource
		owner = ctp.Resource.Owner
	}

	// Part 2: Recompute a CTP from TP create / modify / delete, or create a new CTP from existing TPs:
	latestTrafficPermissions, err := computeNewTrafficPermissions(ctx, rt, ctpID, oldResource)
	if err != nil {
		rt.Logger.Error("error calculating computed permissions", "error", err)
		return err
	}

	if ctp != nil && proto.Equal(ctp.Data, latestTrafficPermissions) {
		// there are no changes to the computed traffic permissions, and we can return early
		rt.Logger.Trace("no new computed traffic permissions")
		return nil
	}
	newCTPData, err := anypb.New(latestTrafficPermissions)
	if err != nil {
		rt.Logger.Error("error marshalling latest traffic permissions", "error", err)
		writeFailedStatus(ctx, rt, oldResource, nil, err.Error())
		return err
	}
	rt.Logger.Trace("writing computed traffic permissions")
	rsp, err := rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:    req.ID,
			Data:  newCTPData,
			Owner: owner,
		},
	})
	if err != nil || rsp.Resource == nil {
		rt.Logger.Error("error writing new computed traffic permissions", "error", err)
		writeFailedStatus(ctx, rt, oldResource, nil, err.Error())
		return err
	} else {
		rt.Logger.Trace("new computed traffic permissions were successfully written")
	}
	newStatus := &pbresource.Status{
		ObservedGeneration: rsp.Resource.Generation,
		Conditions: []*pbresource.Condition{
			ConditionComputed(req.ID.Name, latestTrafficPermissions.IsDefault),
		},
	}
	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     rsp.Resource.Id,
		Key:    ControllerID,
		Status: newStatus,
	})
	return err
}

func writeFailedStatus(ctx context.Context, rt controller.Runtime, ctp *pbresource.Resource, tp *pbresource.ID, errDetail string) error {
	if ctp == nil {
		return nil
	}
	newStatus := &pbresource.Status{
		ObservedGeneration: ctp.Generation,
		Conditions: []*pbresource.Condition{
			ConditionFailedToCompute(ctp.GetId().GetName(), tp.GetName(), errDetail),
		},
	}
	_, err := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     ctp.GetId(),
		Key:    ControllerID,
		Status: newStatus,
	})
	return err
}

// computeNewTrafficPermissions will use all associated Traffic Permissions to create new ComputedTrafficPermissions data
func computeNewTrafficPermissions(ctx context.Context, rt controller.Runtime, ctpID *pbresource.ID, ctp *pbresource.Resource) (*pbauth.ComputedTrafficPermissions, error) {
	// Part 1: Get all TPs that apply to workload identity
	// Get already associated WorkloadIdentities/CTPs for reconcile requests:
	trackedTPs, err := cache.ListDecoded[*pbauth.TrafficPermissions](rt.Cache, pbauth.TrafficPermissionsType, ctpForTPIndexName, ctpID)
	if err != nil {
		return nil, err
	}
	if len(trackedTPs) > 0 {
		rt.Logger.Trace("got tracked traffic permissions for CTP", "tps:", trackedTPs)
	} else {
		rt.Logger.Trace("found no tracked traffic permissions for CTP")
	}
	ap := make([]*pbauth.Permission, 0)
	dp := make([]*pbauth.Permission, 0)
	isDefault := true
	var boundRefs []*pbresource.Reference
	for _, t := range trackedTPs {
		isDefault = false
		if t.Data.Action == pbauth.Action_ACTION_ALLOW {
			ap = append(ap, t.Data.Permissions...)
		} else {
			dp = append(dp, t.Data.Permissions...)
		}
		boundRefs = append(boundRefs, resource.ReferenceFromReferenceOrID(t.Id))
	}
	return &pbauth.ComputedTrafficPermissions{
		AllowPermissions: ap,
		DenyPermissions:  dp,
		IsDefault:        isDefault,
		BoundReferences:  boundRefs,
	}, nil
}
