// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions/expander"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TrafficPermissionsMapper is used to map a watch event for a TrafficPermissions resource and translate
// it to a ComputedTrafficPermissions resource which contains the effective permissions
// from all referencing TrafficPermissions resources.
type TrafficPermissionsMapper interface {
	// MapTrafficPermissions will take a TrafficPermission resource and return controller requests for all
	// ComputedTrafficPermissions associated with that TrafficPermission.
	MapTrafficPermissions(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// UntrackTrafficPermissions instructs the Mapper to forget about the TrafficPermission.
	UntrackTrafficPermissions(*pbresource.ID)

	// GetTrafficPermissionsForCTP returns the tracked TrafficPermissions that are used to create a CTP
	GetTrafficPermissionsForCTP(*pbresource.ID) []*pbresource.Reference
}

// Controller creates a controller for automatic ComputedTrafficPermissions management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller(mapper TrafficPermissionsMapper, sgExpander expander.SamenessGroupExpander) *controller.Controller {
	if mapper == nil {
		panic("TrafficPermissionsMapper is required for TrafficPermissionsController constructor")
	}
	if sgExpander == nil {
		panic("SamenessGroupExpander is required for TrafficPermissionsController constructor")
	}

	samenessGroupIndex := GetSamenessGroupIndex()

	boundRefsMapper := dependency.CacheListMapper(pbauth.ComputedTrafficPermissionsType, BoundRefsIndexName)

	// Maps incoming PartitionTrafficPermissions to ComputedTrafficPermissions requests by prefix searching
	// the CTP's tenancy.
	ptpToCtpMapper := func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		iter, err := rt.Cache.ListIterator(pbauth.ComputedTrafficPermissionsType, "id", &pbresource.Reference{
			Type: pbauth.ComputedTrafficPermissionsType,
			Tenancy: &pbresource.Tenancy{
				Partition: res.Id.Tenancy.GetPartition(),
			},
		}, index.IndexQueryOptions{Prefix: true})
		if err != nil {
			return nil, err
		}

		var reqs []controller.Request
		for res := iter.Next(); res != nil; res = iter.Next() {
			reqs = append(reqs, controller.Request{ID: res.Id})
		}

		return reqs, nil
	}
	// Maps incoming NamespaceTrafficPermissions to ComputedTrafficPermissions requests by prefix searching
	// the CTP's tenancy.
	ntpToCtpMapper := func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		iter, err := rt.Cache.ListIterator(pbauth.ComputedTrafficPermissionsType, "id", &pbresource.Reference{
			Type:    pbauth.ComputedTrafficPermissionsType,
			Tenancy: res.Id.Tenancy,
		}, index.IndexQueryOptions{Prefix: true})
		if err != nil {
			return nil, err
		}

		var reqs []controller.Request
		for res := iter.Next(); res != nil; res = iter.Next() {
			reqs = append(reqs, controller.Request{ID: res.Id})
		}

		return reqs, nil
	}

	ctrl := controller.NewController(StatusKey,
		pbauth.ComputedTrafficPermissionsType,
		boundRefsIndex).
		WithWatch(pbauth.WorkloadIdentityType,
			dependency.ReplaceType(pbauth.ComputedTrafficPermissionsType)).
		WithWatch(pbauth.TrafficPermissionsType,
			dependency.MultiMapper(boundRefsMapper, mapper.MapTrafficPermissions),
			samenessGroupIndex).
		WithWatch(pbauth.PartitionTrafficPermissionsType,
			dependency.MultiMapper(boundRefsMapper, ptpToCtpMapper),
			indexPtpByTenancy()).
		WithWatch(pbauth.NamespaceTrafficPermissionsType,
			dependency.MultiMapper(boundRefsMapper, ntpToCtpMapper),
			indexNtpByTenancy()).
		WithReconciler(&reconciler{mapper: mapper, sgExpander: sgExpander})

	return registerEnterpriseControllerWatchers(ctrl)
}

type reconciler struct {
	mapper     TrafficPermissionsMapper
	sgExpander expander.SamenessGroupExpander
}

// Reconcile will reconcile one ComputedTrafficPermissions (CTP) in response to some event.
// Events include adding, modifying or deleting a WorkloadIdentity or TrafficPermission or SamenessGroupType.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)

	// The bound reference collector is supposed to aggregate all
	// references to resources that influence the production of
	// a ComputedTrafficPermissions resource.
	//
	// We only add a reference to the collector if the following are ALL true:
	//
	// - We load the resource for some reason.
	// - The resource is found.
	// - We decided to use the information in that resource to produce
	//   ComputedTrafficPermissions.
	brc := resource.NewBoundReferenceCollector()

	ctpID := req.ID
	oldCTPData, err := resource.GetDecodedResource[*pbauth.ComputedTrafficPermissions](ctx, rt.Client, ctpID)
	if err != nil {
		rt.Logger.Error("error retrieving computed permissions", "error", err)
		return err
	}

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

	sgMap, err := r.sgExpander.List(ctx, rt, req)
	if err != nil {
		rt.Logger.Error("error retrieving sameness groups", err.Error())
		return err
	}

	trafficPermissionBuilder := newTrafficPermissionsBuilder(r.sgExpander, sgMap, brc)
	var tpResources []*pbresource.Resource

	// Part 2: Recompute a CTP from TP create / modify / delete, or create a new CTP from existing TPs:
	trackedTPs := r.mapper.GetTrafficPermissionsForCTP(ctpID)
	if len(trackedTPs) > 0 {
		rt.Logger.Trace("got tracked traffic permissions for CTP", "tps:", trackedTPs)
	} else {
		rt.Logger.Trace("found no tracked traffic permissions for CTP")
	}

	for _, t := range trackedTPs {
		rsp, err := resource.GetDecodedResource[*pbauth.TrafficPermissions](ctx, rt.Client, resource.IDFromReference(t))
		if err != nil {
			rt.Logger.Error("error reading traffic permissions resource for computation", "error", err)
			writeFailedStatus(ctx, rt, oldResource, resource.IDFromReference(t), err.Error())
			return err
		}
		if rsp == nil {
			rt.Logger.Trace("untracking deleted TrafficPermissions", "traffic-permissions-name", t.Name)
			r.mapper.UntrackTrafficPermissions(resource.IDFromReference(t))
			continue
		}
		track(trafficPermissionBuilder, rsp)
		tpResources = append(tpResources, rsp.Resource)
	}

	// Fetch partition traffic permissions for ctp(workload identity)'s tenancy
	ptps, err := cache.ListDecoded[*pbauth.PartitionTrafficPermissions](
		rt.Cache,
		pbauth.PartitionTrafficPermissionsType,
		TenancyIndexName,
		&pbresource.Tenancy{Partition: ctpID.Tenancy.GetPartition()},
	)
	if err != nil {
		rt.Logger.Error("error reading partitioned traffic permissions resource for computation", "error", err)
		writeFailedStatus(ctx, rt, oldResource, nil, err.Error())
		return err
	}
	for _, ptp := range ptps {
		track(trafficPermissionBuilder, ptp)
		tpResources = append(tpResources, ptp.Resource)
	}

	// Fetch namespace traffic permissions for ctp(workload identity)'s tenancy
	ntps, err := cache.ListDecoded[*pbauth.NamespaceTrafficPermissions](
		rt.Cache,
		pbauth.NamespaceTrafficPermissionsType,
		TenancyIndexName,
		ctpID.Tenancy,
	)
	if err != nil {
		rt.Logger.Error("error reading namespaced traffic permissions resource for computation", "error", err)
		writeFailedStatus(ctx, rt, oldResource, nil, err.Error())
		return err
	}
	for _, ntp := range ntps {
		track(trafficPermissionBuilder, ntp)
		tpResources = append(tpResources, ntp.Resource)
	}

	latestComputedTrafficPermissions, missing := trafficPermissionBuilder.build()

	newCTPResource := oldResource

	allMissing := missingForCTP(missing)

	if (oldCTPData == nil) || (!proto.Equal(oldCTPData.Data, latestComputedTrafficPermissions)) {
		rt.Logger.Trace("no new computed traffic permissions")

		// We can't short circuit here because we always need to update statuses.
		newCTPData, err := anypb.New(latestComputedTrafficPermissions)
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
		if err != nil {
			rt.Logger.Error("error writing new computed traffic permissions", "error", err)
			writeFailedStatus(ctx, rt, oldResource, nil, err.Error())
			return err
		} else {
			rt.Logger.Trace("new computed traffic permissions were successfully written")
		}
		newCTPResource = rsp.Resource
	}

	if len(allMissing) > 0 {
		return writeMissingSgStatuses(ctx, rt, req, allMissing, newCTPResource, missing, tpResources)
	}

	return writeComputedStatuses(ctx, rt, req, newCTPResource, latestComputedTrafficPermissions.IsDefault, tpResources)
}

func writeComputedStatuses(ctx context.Context, rt controller.Runtime, req controller.Request, ctpResource *pbresource.Resource, isDefault bool,
	trackedTPs []*pbresource.Resource) error {
	for _, tp := range trackedTPs {
		err := writeStatusWithConditions(ctx, rt, tp,
			[]*pbresource.Condition{ConditionComputedTrafficPermission()})
		if err != nil {
			return err
		}
	}
	condition := ConditionComputed(req.ID.Name, isDefault)
	return writeStatusWithConditions(ctx, rt, ctpResource, []*pbresource.Condition{condition})
}

func writeMissingSgStatuses(ctx context.Context, rt controller.Runtime, req controller.Request, allMissing []string, newCTPResource *pbresource.Resource,
	missing map[resource.ReferenceKey]missingSamenessGroupReferences, tpResources []*pbresource.Resource) error {

	condition := ConditionMissingSamenessGroup(req.ID.Tenancy.Partition, allMissing)

	rt.Logger.Trace("writing missing sameness groups status")
	err := writeStatusWithConditions(ctx, rt, newCTPResource, []*pbresource.Condition{condition})
	if err != nil {
		return err
	}
	// writing status to traffic permissions
	for _, sgRefs := range missing {
		if len(sgRefs.samenessGroups) == 0 {
			err := writeStatusWithConditions(ctx, rt, sgRefs.resource,
				[]*pbresource.Condition{ConditionComputedTrafficPermission()})
			if err != nil {
				return err
			}
			continue
		}
		conditionTp := ConditionMissingSamenessGroup(req.ID.Tenancy.Partition, sgRefs.samenessGroups)
		err := writeStatusWithConditions(ctx, rt, sgRefs.resource, []*pbresource.Condition{conditionTp})
		if err != nil {
			return err
		}
	}
	for _, trackedTp := range tpResources {
		if _, ok := missing[resource.NewReferenceKey(trackedTp.Id)]; ok {
			continue
		}
		err := writeStatusWithConditions(ctx, rt, trackedTp,
			[]*pbresource.Condition{ConditionComputedTrafficPermission()})
		if err != nil {
			return err
		}
	}
	return nil
}

func writeStatusWithConditions(ctx context.Context, rt controller.Runtime, res *pbresource.Resource,
	conditions []*pbresource.Condition) error {

	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions:         conditions,
	}

	if resource.EqualStatus(res.Status[StatusKey], newStatus, false) {
		rt.Logger.Trace("old status is same as new status. skipping write", "resource", res.Id)
		return nil
	}

	_, err := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    StatusKey,
		Status: newStatus,
	})
	return err
}

func writeFailedStatus(ctx context.Context, rt controller.Runtime, ctp *pbresource.Resource, tp *pbresource.ID, errDetail string) error {
	if ctp == nil {
		return nil
	}
	conditions := []*pbresource.Condition{
		ConditionFailedToCompute(ctp.Id.Name, tp.GetName(), errDetail),
	}
	return writeStatusWithConditions(ctx, rt, ctp, conditions)
}
