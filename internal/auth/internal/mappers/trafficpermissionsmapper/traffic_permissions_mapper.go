// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissionsmapper

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TrafficPermissionsMapper struct {
	lock     sync.Mutex
	mapper   *bimapper.Mapper
	liveCTPs map[string]*pbresource.ID
}

func New() *TrafficPermissionsMapper {
	return &TrafficPermissionsMapper{
		lock:     sync.Mutex{},
		mapper:   bimapper.New(types.ComputedTrafficPermissionsType, types.TrafficPermissionsType),
		liveCTPs: make(map[string]*pbresource.ID, 0),
	}
}

//  TrafficPermissionsMapper functions

func (tm *TrafficPermissionsMapper) getCTPForWorkloadIdentity(destination string) *pbresource.ID {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	return tm.liveCTPs[destination]
}

func (tm *TrafficPermissionsMapper) trackTPForWorkloadIdentity(tp *pbresource.ID, wi *pbresource.ID) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	newTPsForWI := []resource.ReferenceOrID{tp}
	for _, mtp := range tm.mapper.LinkIDsForItem(wi) {
		newTPsForWI = append(newTPsForWI, mtp)
	}
	tm.mapper.TrackItem(wi, newTPsForWI)
}

func (tm *TrafficPermissionsMapper) MapTrafficPermissions(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var tp pbauth.TrafficPermissions
	err := res.Data.UnmarshalTo(&tp)
	if err != nil {
		return nil, resource.NewErrDataParse(&tp, err)
	}
	// get new CTP associations based on destination
	if len(tp.Destination.IdentityName) == 0 {
		// this error should never happen if validation is working
		return nil, types.ErrWildcardNotSupported
	}
	// Does the identity exist in our mappings?
	ctp := tm.getCTPForWorkloadIdentity(tp.Destination.IdentityName)
	if ctp == nil {
		// if not, make a new ID and track it
		ctp = &pbresource.ID{
			Name:    tp.Destination.IdentityName,
			Type:    types.ComputedTrafficPermissionsType,
			Tenancy: res.Id.Tenancy,
		}
		tm.trackTPForWorkloadIdentity(res.Id, ctp)
		return nil, nil
	}
	tm.trackTPForWorkloadIdentity(res.Id, ctp)
	requests := []controller.Request{{ID: ctp}}
	// add already associated WorkloadIdentities/CTPs for reconcile requests:
	for _, mappedWI := range tm.mapper.ItemIDsForLink(res.Id) {
		if mappedWI.Name != ctp.Name {
			requests = append(requests, controller.Request{ID: mappedWI})
		}
	}
	return requests, nil
}

func (tm *TrafficPermissionsMapper) UntrackTrafficPermissions(tp *pbresource.ID) {
	tm.mapper.UntrackLink(tp)
}

func (tm *TrafficPermissionsMapper) UntrackWorkloadIdentity(ctx context.Context, rt controller.Runtime, ctp *pbresource.ID) error {
	// check if there are any remaining TPs for the WI, and remove it from the map if there are none
	tps := tm.mapper.LinkIDsForItem(ctp)
	// prune any dead TPs
	pruned := 0
	for _, tp := range tps {
		res, err := resource.GetDecodedResource[*pbauth.TrafficPermissions](ctx, rt.Client, tp)
		if err != nil {
			return err
		}
		if res == nil {
			tm.UntrackTrafficPermissions(tp)
			pruned += 1
		}
	}
	if len(tps) == 0 || pruned == len(tps) {
		// remove from bimapper
		tm.mapper.UntrackItem(ctp)
	}
	// remove from tracked WIs
	delete(tm.liveCTPs, ctp.Name)
	return nil
}

func (tm *TrafficPermissionsMapper) TrackCTPForWorkloadIdentity(ctp *pbresource.ID, wi *pbresource.ID) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	tm.liveCTPs[wi.Name] = ctp
}

func (tm *TrafficPermissionsMapper) GetTrafficPermissionsForCTP(ctp *pbresource.ID) []*pbresource.Reference {
	return tm.mapper.LinkRefsForItem(ctp)
}
