// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TrafficPermissionsMapper is used to map a watch event for a TrafficPermissions resource and translate
// it to a ComputedTrafficPermissions resource which contains the effective permissions
// from all referencing TrafficPermissions resources.
type TrafficPermissionsMapper interface {
	// MapTrafficPermissions will take a TrafficPermission resource and return controller requests for all
	// ComputedTrafficPermissions associated with that TrafficPermission.
	MapTrafficPermissions(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// TrackCTPForWorkloadIdentity instructs the Mapper to track the WorkloadIdentity. If the WorkloadIdentity is already
	// being tracked, it is a no-op.
	TrackCTPForWorkloadIdentity(*pbresource.ID, *pbresource.ID)

	// UntrackWorkloadIdentity instructs the Mapper to forget about the WorkloadIdentity and associated
	// ComputedTrafficPermission.
	UntrackWorkloadIdentity(context.Context, controller.Runtime, *pbresource.ID) error

	// UntrackTrafficPermissions instructs the Mapper to forget about the TrafficPermission.
	UntrackTrafficPermissions(*pbresource.ID)

	// GetTrafficPermissionsForCTP returns the tracked TrafficPermissions that are used to create a CTP
	GetTrafficPermissionsForCTP(id *pbresource.ID) []*pbresource.Reference
}

// Controller creates a controller for automatic ComputedTrafficPermissions management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller(mapper TrafficPermissionsMapper) controller.Controller {
	if mapper == nil {
		panic("No TrafficPermissionsMapper was provided to the TrafficPermissionsController constructor")
	}

	return controller.ForType(types.ComputedTrafficPermissionsType).
		WithWatch(types.WorkloadIdentityType, controller.ReplaceType(types.ComputedTrafficPermissionsType)).
		WithWatch(types.TrafficPermissionsType, mapper.MapTrafficPermissions).
		WithReconciler(&reconciler{mapper: mapper})
}

type reconciler struct {
	mapper TrafficPermissionsMapper
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
	wi := &pbresource.ID{
		Type:    types.WorkloadIdentityType,
		Tenancy: ctpID.Tenancy,
		Name:    ctpID.Name,
	}
	workloadIdentity, err := resource.GetDecodedResource[*pbauth.WorkloadIdentity](ctx, rt.Client, wi)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding Workload Identity", "error", err)
		return err
	}
	if workloadIdentity == nil || workloadIdentity.Resource == nil {
		rt.Logger.Trace("workload identity has been deleted")
		// The workload identity was deleted, so we need to update the mapper to tell it to
		// stop tracking this workload identity, and clean up the associated CTP
		if err := r.mapper.UntrackWorkloadIdentity(ctx, rt, ctpID); err != nil {
			return err
		}
		return nil
	}

	// Check if CTP exists:
	oldCTPData, err := resource.GetDecodedResource[*pbauth.ComputedTrafficPermissions](ctx, rt.Client, ctpID)
	if err != nil {
		rt.Logger.Error("error retrieving computed permissions", "error", err)
		return err
	}
	if oldCTPData == nil {
		// CTP does not yet exist, so we need to make a new one
		rt.Logger.Trace("creating new computed traffic permissions for new workload identity")
		r.mapper.TrackCTPForWorkloadIdentity(ctpID, workloadIdentity.Resource.Id)
	}

	// Part 2: Recompute a CTP from TP create / modify / delete, or create a new CTP from existing TPs:
	latestTrafficPermissions, err := computeNewTrafficPermissions(ctx, rt, r.mapper, ctpID)
	if err != nil {
		rt.Logger.Error("error calculating computed permissions", "error", err)
		return err
	}

	if oldCTPData != nil && proto.Equal(oldCTPData.Data, latestTrafficPermissions) {
		// there are no changes to the computed traffic permissions, and we can return early
		return nil
	}
	newCTPData, err := anypb.New(latestTrafficPermissions)
	if err != nil {
		rt.Logger.Error("error marshalling latest traffic permissions", "error", err)
		return err
	}
	rt.Logger.Trace("writing new computed traffic permissions with ID", "id:", req.ID)
	rsp, err := rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:    req.ID,
			Data:  newCTPData,
			Owner: workloadIdentity.Resource.Id,
		},
	})
	if err != nil || rsp.Resource == nil {
		rt.Logger.Error("error writing new computed traffic permissions", "error", err)
		return err
	} else {
		rt.Logger.Trace("new computed traffic permissions were successfully written")
	}
	newStatus := &pbresource.Status{
		ObservedGeneration: rsp.Resource.Generation,
		Conditions: []*pbresource.Condition{
			ConditionComputed(req.ID.Name),
		},
	}
	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     rsp.Resource.Id,
		Key:    StatusKey,
		Status: newStatus,
	})
	return err
}

// computeNewTrafficPermissions will use all associated Traffic Permissions to create new ComputedTrafficPermissions data
func computeNewTrafficPermissions(ctx context.Context, rt controller.Runtime, wm TrafficPermissionsMapper, ctpID *pbresource.ID) (*pbauth.ComputedTrafficPermissions, error) {
	// Part 1: Get all TPs that apply to workload identity
	// Get already associated WorkloadIdentities/CTPs for reconcile requests:
	trackedTPs := wm.GetTrafficPermissionsForCTP(ctpID)
	rt.Logger.Trace("got tracked TPs for CTP", "ctp:", ctpID.Name, "tps:", trackedTPs)
	ap := make([]*pbauth.Permission, 0)
	dp := make([]*pbauth.Permission, 0)
	for _, tp := range trackedTPs {
		rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: resource.IDFromReference(tp)})
		switch {
		case status.Code(err) == codes.NotFound:
			rt.Logger.Trace("untracking deleted TrafficPermissions", "traffic-permissions-name", tp.Name)
			wm.UntrackTrafficPermissions(resource.IDFromReference(tp))
			continue
		case err != nil:
			rt.Logger.Error("error reading traffic permissions resource for computation", "error", err)
			return nil, err
		}
		var tp pbauth.TrafficPermissions
		err = rsp.Resource.Data.UnmarshalTo(&tp)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to parse traffic permissions data")
		}
		if tp.Action == pbauth.Action_ACTION_ALLOW {
			ap = append(ap, tp.Permissions...)
		} else {
			dp = append(dp, tp.Permissions...)
		}
	}
	return &pbauth.ComputedTrafficPermissions{AllowPermissions: ap, DenyPermissions: dp}, nil
}
