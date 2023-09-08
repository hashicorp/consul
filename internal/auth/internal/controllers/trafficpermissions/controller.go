// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/auth/internal/mappers/trafficpermissionsmapper"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Mapper is used to map a watch event for a TrafficPermission resource and translate
// it to a ComputedTrafficPermissions resource which contains the effective permissions
// from the TrafficPermission resource.
type WorkloadIdentityMapper interface {
	// MapTrafficPermission will take a TrafficPermission resource and return controller requests for all
	// ComputedTrafficPermissions associated with that TrafficPermission.
	MapTrafficPermissions(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// UntrackWorkloadIdentity instructs the Mapper to track the WorkloadIdentity. If the WorkloadIdentity is already
	// being tracked, it is a no-op.
	TrackWorkloadIdentity(*pbresource.ID)

	// UntrackWorkloadIdentity instructs the Mapper to forget about the WorkloadIdentity and associated
	// ComputedTrafficPermission.
	UntrackWorkloadIdentity(*pbresource.ID)

	// UntrackTrafficPermission instructs the Mapper to forget about the TrafficPermission.
	UntrackTrafficPermissions(*pbresource.ID)

	// Get a new set of computed permissions
	ComputeNewTrafficPermissions(context.Context, controller.Runtime, *pbresource.ID) (*pbauth.ComputedTrafficPermissions, error)
}

// Controller creates a controller for automatic ComputedTrafficPermissions management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller(mapper WorkloadIdentityMapper) controller.Controller {
	if mapper == nil {
		panic("No TrafficPermissionsMapper was provided to the TrafficPermissionsController constructor")
	}

	return controller.ForType(types.ComputedTrafficPermissionsType).
		WithWatch(types.WorkloadIdentityType, controller.ReplaceType(types.ComputedTrafficPermissionsType)).
		WithWatch(types.TrafficPermissionsType, mapper.MapTrafficPermissions).
		WithWatch(types.NamespaceTrafficPermissionsType, mapper.MapTrafficPermissions).
		WithWatch(types.PartitionTrafficPermissionsType, mapper.MapTrafficPermissions).
		WithReconciler(&reconciler{mapper: mapper})
}

type reconciler struct {
	mapper WorkloadIdentityMapper
}

// Reconcile will reconcile one ComputedTrafficPermission (CTP) in response to some event.
// Events include adding, modifying or deleting a WorkloadIdentity or TrafficPermission.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)
	rt.Logger.Trace("reconciling computed traffic permissions")
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
	workloadIdentity, err := trafficpermissionsmapper.LookupWorkloadIdentityByName(ctx, rt, ctpID, ctpID.Name)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding Workload Identity", "error", err)
		return err
	}
	if workloadIdentity == nil {
		rt.Logger.Trace("workload identity has been deleted")
		// The workload identity was deleted, so we need to update the mapper to tell it to
		// stop tracking this workload identity, and clean up the associated CTP
		r.mapper.UntrackWorkloadIdentity(&pbresource.ID{
			Type:    types.WorkloadIdentityType,
			Tenancy: ctpID.Tenancy,
			Name:    ctpID.Name,
		})
		return nil
	}
	rt.Logger.Trace("Got active WorkloadIdentity in CTP reconciler", "name", workloadIdentity.Id.Name)

	// Check if CTP exists:
	oldCTPData, err := getCTPData(ctx, rt, ctpID)
	if err != nil {
		rt.Logger.Error("error retrieving computed permissions", "error", err)
		return err
	}
	// make sure we are tracking the WorkloadIdentity
	r.mapper.TrackWorkloadIdentity(workloadIdentity.Id)

	// Part 2: Recompute a CTP from TP create / modify / delete, or create a new CTP from existing TPs:
	latestTrafficPermissions, err := r.mapper.ComputeNewTrafficPermissions(ctx, rt, workloadIdentity.Id)
	if err != nil {
		rt.Logger.Error("error calculating computed permissions", "error", err)
		return err
	}

	if oldCTPData != nil && proto.Equal(oldCTPData.ctp, latestTrafficPermissions) {
		// there are no changes to the computed traffic permissions, and we can return early
		return nil
	}

	if oldCTPData == nil {
		// CTP does not yet exist, so we need to make a new one
		rt.Logger.Trace("creating new computed traffic permissions for workload identity")
		// First encode the data as an Any type.
		newCTPData, err := anypb.New(latestTrafficPermissions)
		if err != nil {
			rt.Logger.Error("error marshalling latest traffic permissions", "error", err)
			return err
		}
		// Write the new CTP.
		_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
			Resource: &pbresource.Resource{
				Id:   ctpID,
				Data: newCTPData,
			},
		})
	}

	status := &pbresource.Status{
		ObservedGeneration: workloadIdentity.Generation,
		Conditions: []*pbresource.Condition{
			ConditionComputed(workloadIdentity.Id.Name, ""),
		},
	}
	// If the status is unchanged then we should return and avoid the unnecessary write
	if resource.EqualStatus(workloadIdentity.Status[StatusKey], status, false) {
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     workloadIdentity.Id,
		Key:    StatusKey,
		Status: status,
	})
	return nil
}

type ctpData struct {
	resource *pbresource.Resource
	ctp      *pbauth.ComputedTrafficPermissions
}

// getCTPData will read the computed traffic permissions with the given
// ID and unmarshal the Data field. The return value is a struct that
// contains the retrieved resource as well as the unmsashalled form.
// If the resource doesn't  exist, nil will be returned. Any other error
// either with retrieving the resource or unmarshalling it will cause the
// error to be returned to the caller.
func getCTPData(ctx context.Context, rt controller.Runtime, id *pbresource.ID) (*ctpData, error) {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: id})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	var ctp pbauth.ComputedTrafficPermissions
	err = rsp.Resource.Data.UnmarshalTo(&ctp)
	if err != nil {
		return nil, resource.NewErrDataParse(&ctp, err)
	}

	return &ctpData{resource: rsp.Resource, ctp: &ctp}, nil
}
