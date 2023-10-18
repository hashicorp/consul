// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/auth/internal/indexers"
	"github.com/hashicorp/consul/internal/controller"
	cacheindexers "github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TrafficPermissionsMapper = controller.DependencyMapper

// Controller creates a controller for automatic ComputedTrafficPermissions management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller(mapper TrafficPermissionsMapper) controller.Controller {
	if mapper == nil {
		panic("No TrafficPermissionsMapper was provided to the TrafficPermissionsController constructor")
	}

	boundRefsIndex := cacheindexers.RefIndex[*pbauth.ComputedTrafficPermissions](
		func(res *resource.DecodedResource[*pbauth.ComputedTrafficPermissions]) []*pbresource.Reference {
			return res.Data.BoundReferences
		},
	)

	return controller.ForType(pbauth.ComputedTrafficPermissionsType).
		WithIndex(pbauth.ComputedTrafficPermissionsType, "bound_references", boundRefsIndex).
		WithWatch(pbauth.WorkloadIdentityType, controller.ReplaceType(pbauth.ComputedTrafficPermissionsType)).
		// This cache index is being kept to allow listing off all TrafficPermissions linked to a ComputedTrafficPermissions
		// resource. It will not be used for event mapping.
		WithIndex(pbauth.TrafficPermissionsType, "computed", indexers.TrafficPermissionsIndex()).
		// We need to handle the case where a TrafficPermissions Identity has been altered. To do so we will
		// store references to all TrafficPermissions included in the ComputedTrafficPermissions within the
		// resource and keep an index on that
		WithWatch(pbauth.TrafficPermissionsType, controller.MultiMapper(
			// generates events for all ComputedTrafficPermissions that current reference this TrafficPermissions
			controller.CacheListMapper(pbauth.ComputedTrafficPermissionsType, "bound_references"),
			// generates events for the ComputedTrafficPermissions that this resource should now map to.
			mapper,
		)).
		WithReconciler(&reconciler{})
}

type reconciler struct {
}

// Reconcile will reconcile one ComputedTrafficPermission (CTP) in response to some event.
// Events include adding, modifying or deleting a WorkloadIdentity or TrafficPermission.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)
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
	workloadIdentity, err := resource.GetDecodedResource[*pbauth.WorkloadIdentity](ctx, rt.Client, wi)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding Workload Identity", "error", err)
		return err
	}
	if workloadIdentity == nil || workloadIdentity.Resource == nil {
		rt.Logger.Trace("workload identity has been deleted")
		return nil
	}

	// Check if CTP exists:
	oldCTPData, err := resource.GetDecodedResource[*pbauth.ComputedTrafficPermissions](ctx, rt.Client, ctpID)
	if err != nil {
		rt.Logger.Error("error retrieving computed permissions", "error", err)
		return err
	}
	var oldResource *pbresource.Resource
	var owner *pbresource.ID
	if oldCTPData == nil {
		// CTP does not yet exist, so we need to make a new one
		rt.Logger.Trace("creating new computed traffic permissions for workload identity")
		owner = workloadIdentity.Resource.Id
	} else {
		oldResource = oldCTPData.Resource
		owner = oldCTPData.Resource.Owner
	}

	// Part 2: Recompute a CTP from TP create / modify / delete, or create a new CTP from existing TPs:
	latestTrafficPermissions, err := computeNewTrafficPermissions(ctx, rt, ctpID, oldResource)
	if err != nil {
		rt.Logger.Error("error calculating computed permissions", "error", err)
		return err
	}

	if oldCTPData != nil && proto.Equal(oldCTPData.Data, latestTrafficPermissions) {
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
		Key:    StatusKey,
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
			ConditionFailedToCompute(ctp.Id.Name, tp.Name, errDetail),
		},
	}
	_, err := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     ctp.Id,
		Key:    StatusKey,
		Status: newStatus,
	})
	return err
}

// computeNewTrafficPermissions will use all associated Traffic Permissions to create new ComputedTrafficPermissions data
func computeNewTrafficPermissions(ctx context.Context, rt controller.Runtime, ctpID *pbresource.ID, ctp *pbresource.Resource) (*pbauth.ComputedTrafficPermissions, error) {
	// Part 1: Get all TPs that apply to workload identity
	// Get already associated WorkloadIdentities/CTPs for reconcile requests:
	trackedTPs, err := rt.Cache.List(pbauth.TrafficPermissionsType, "computed", ctpID)
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
		rsp, err := resource.GetDecodedResource[*pbauth.TrafficPermissions](ctx, rt.Client, t.GetId())
		if err != nil {
			rt.Logger.Error("error reading traffic permissions resource for computation", "error", err)
			writeFailedStatus(ctx, rt, ctp, t.GetId(), err.Error())
			return nil, err
		}
		// The TrafficPermissions doesn't exist or is no longer linked with this ComputedTrafficPermissions
		// The second case may happen if an update event for the TrafficPermissions happened after the
		// cache lookup at the beginning of this function but before we retrieved the latest resource.
		if rsp == nil || rsp.Data.Destination.IdentityName != ctpID.Name {
			continue
		}
		isDefault = false

		boundRefs = append(boundRefs, resource.ReferenceFromReferenceOrID(rsp.Resource.Id))
		if rsp.Data.Action == pbauth.Action_ACTION_ALLOW {
			ap = append(ap, rsp.Data.Permissions...)
		} else {
			dp = append(dp, rsp.Data.Permissions...)
		}
	}
	return &pbauth.ComputedTrafficPermissions{
		AllowPermissions: ap,
		DenyPermissions:  dp,
		IsDefault:        isDefault,
		BoundReferences:  boundRefs,
	}, nil
}
