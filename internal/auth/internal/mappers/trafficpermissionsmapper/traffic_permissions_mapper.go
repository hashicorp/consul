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
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TrafficPermissionsMapper struct {
	lock   sync.Mutex
	mapper *bimapper.Mapper
}

func New() *TrafficPermissionsMapper {
	return &TrafficPermissionsMapper{
		lock:   sync.Mutex{},
		mapper: bimapper.New(pbauth.TrafficPermissionsType, pbauth.ComputedTrafficPermissionsType),
	}
}

//  TrafficPermissionsMapper functions

func (tm *TrafficPermissionsMapper) MapTrafficPermissions(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
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
	newCTP := &pbresource.ID{
		Name:    tp.Destination.IdentityName,
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: res.Id.Tenancy,
	}
	requests := []controller.Request{{ID: newCTP}}
	// add already associated WorkloadIdentities/CTPs for reconcile requests:
	oldCTPs := tm.mapper.LinkIDsForItem(res.Id)
	for _, mappedWI := range oldCTPs {
		if mappedWI.Name != newCTP.Name {
			requests = append(requests, controller.Request{ID: mappedWI})
		}
	}
	// re-map traffic permission to new CTP
	tm.mapper.TrackItem(res.Id, []resource.ReferenceOrID{newCTP})
	return requests, nil
}

func (tm *TrafficPermissionsMapper) UntrackTrafficPermissions(tp *pbresource.ID) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	tm.mapper.UntrackItem(tp)
}

func (tm *TrafficPermissionsMapper) GetTrafficPermissionsForCTP(ctp *pbresource.ID) []*pbresource.Reference {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	return tm.mapper.ItemRefsForLink(ctp)
}
