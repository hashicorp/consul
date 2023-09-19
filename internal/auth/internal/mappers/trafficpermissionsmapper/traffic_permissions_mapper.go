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
	lock                     sync.Mutex
	mapper                   *bimapper.Mapper
	workloadIdentityToCTPMap map[string]*pbresource.ID
}

func New() *TrafficPermissionsMapper {
	return &TrafficPermissionsMapper{
		lock:                     sync.Mutex{},
		mapper:                   bimapper.New(types.ComputedTrafficPermissionsType, types.TrafficPermissionsType),
		workloadIdentityToCTPMap: make(map[string]*pbresource.ID, 0),
	}
}

//  TrafficPermissionsMapper functions

func (tm *TrafficPermissionsMapper) MapTrafficPermissions(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	mappedWIs := make([]controller.Request, 0)
	// get already associated WorkloadIdentities/CTPs for reconcile requests:
	associatedWIs := tm.mapper.ItemIDsForLink(res.Id)
	for _, w := range associatedWIs {
		mappedWIs = append(mappedWIs, controller.Request{ID: w})
	}
	var tp pbauth.TrafficPermissions
	err := res.Data.UnmarshalTo(&tp)
	if err != nil {
		return nil, resource.NewErrDataParse(&tp, err)
	}
	// get new CTP associations based on destination
	if len(tp.Destination.IdentityName) > 0 {
		// Does the identity exist in our mappings?
		ctp := tm.GetCTPForWorkloadIdentity(tp.Destination.IdentityName)
		if ctp != nil {
			tm.mapper.AddLinksForItem(ctp, []resource.ReferenceOrID{res.Id}, false)
			mappedWIs = append(mappedWIs, controller.Request{ID: ctp})
		} else {
			// if not, make a new ID and track it
			ctp = &pbresource.ID{
				Name:    tp.Destination.IdentityName,
				Type:    types.ComputedTrafficPermissionsType,
				Tenancy: res.Id.Tenancy,
			}
			tm.mapper.AddLinksForItem(ctp, []resource.ReferenceOrID{res.Id}, false)
			return nil, nil
		}
	} else {
		// this error should never happen if validation is working
		return nil, types.ErrWildcardNotSupported
	}
	return mappedWIs, nil
}

func (tm *TrafficPermissionsMapper) UntrackTrafficPermissions(tp *pbresource.ID) {
	tm.mapper.UntrackLink(tp)
}

func (tm *TrafficPermissionsMapper) UntrackWorkloadIdentity(ctp *pbresource.ID) {
	// check if there are any remaining TPs for the WI, and remove it from the map if there are none
	tps := tm.mapper.LinkRefsForItem(ctp)
	if len(tps) == 0 {
		// remove from bimapper
		tm.mapper.UntrackItem(ctp)
	}
	// remove from tracked WIs
	delete(tm.workloadIdentityToCTPMap, ctp.Name)
	return
}

func (tm *TrafficPermissionsMapper) TrackCTPForWorkloadIdentity(ctp *pbresource.ID, wi *pbresource.ID) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	tm.workloadIdentityToCTPMap[wi.Name] = ctp
}

func (tm *TrafficPermissionsMapper) GetCTPForWorkloadIdentity(destination string) *pbresource.ID {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	return tm.workloadIdentityToCTPMap[destination]
}

func (tm *TrafficPermissionsMapper) GetTrafficPermissionsForCTP(ctp *pbresource.ID) []*pbresource.Reference {
	return tm.mapper.LinkRefsForItem(ctp)
}
