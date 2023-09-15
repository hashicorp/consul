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
	missingMap               *MissingWorkloadIdentityMapper // indexes on the name of the missing Workload Identity
	workloadIdentityToCTPMap map[string]*pbresource.ID
}

// TODO: We wouldn't need this if we can make a generic bimapper that doesn't clean up dead-end links.
// MissingWorkloadIdentityMapper holds traffic permissions with explicit destinations that are missing an active workload identity.
type MissingWorkloadIdentityMapper struct {
	tpToWorkloadIdentityName map[resource.ReferenceKey]map[string]bool
	workloadIdentityNameToTP map[string]map[resource.ReferenceKey]bool
}

func newMissingWorkloadIdentityMapper() *MissingWorkloadIdentityMapper {
	return &MissingWorkloadIdentityMapper{
		tpToWorkloadIdentityName: make(map[resource.ReferenceKey]map[string]bool),
		workloadIdentityNameToTP: make(map[string]map[resource.ReferenceKey]bool),
	}
}

func (mm *MissingWorkloadIdentityMapper) track(tp *pbresource.ID, name string) {
	tpRef := resource.NewReferenceKey(tp)
	_, ok := mm.workloadIdentityNameToTP[name]
	if !ok {
		mm.workloadIdentityNameToTP[name] = make(map[resource.ReferenceKey]bool)
	}
	mm.workloadIdentityNameToTP[name][tpRef] = true
	_, ok = mm.tpToWorkloadIdentityName[tpRef]
	if !ok {
		mm.tpToWorkloadIdentityName[tpRef] = make(map[string]bool)
	}
	mm.tpToWorkloadIdentityName[tpRef][name] = true
}

func (mm *MissingWorkloadIdentityMapper) untrack(tp *pbresource.ID) {
	tpRef := resource.NewReferenceKey(tp)
	wiNames, ok := mm.tpToWorkloadIdentityName[tpRef]
	if ok {
		for wiName := range wiNames {
			delete(mm.workloadIdentityNameToTP[wiName], tpRef)
		}
	}
	delete(mm.tpToWorkloadIdentityName, tpRef)
}

func New() *TrafficPermissionsMapper {
	return &TrafficPermissionsMapper{
		lock:                     sync.Mutex{},
		mapper:                   bimapper.New(types.ComputedTrafficPermissionsType, types.TrafficPermissionsType),
		missingMap:               newMissingWorkloadIdentityMapper(),
		workloadIdentityToCTPMap: make(map[string]*pbresource.ID, 0),
	}
}

//  TrafficPermissionsMapper functions

func (tm *TrafficPermissionsMapper) MapTrafficPermissions(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	workloadIdentities := make([]controller.Request, 0)
	// get already associated WorkloadIdentities/CTPs for reconcile requests:
	associatedWIs := tm.mapper.ItemIDsForLink(res.Id)
	for _, w := range associatedWIs {
		workloadIdentities = append(workloadIdentities, controller.Request{ID: w})
	}
	var tp pbauth.TrafficPermissions
	err := res.Data.UnmarshalTo(&tp)
	if err != nil {
		return nil, resource.NewErrDataParse(&tp, err)
	}
	newWorkloadIdentities := make([]controller.Request, 0)
	// get new CTP associations based on destination
	if len(tp.Destination.IdentityName) > 0 {
		// Does the identity exist in our mappings?
		ctp, ok := tm.workloadIdentityToCTPMap[tp.Destination.IdentityName]
		if ok {
			tm.trackTrafficPermissionsForCTP(res.Id, ctp)
			newWorkloadIdentities = append(workloadIdentities, controller.Request{ID: ctp})
		} else {
			// if not add to missingWIMap
			tm.missingMap.track(res.Id, tp.Destination.IdentityName)
		}
	} else {
		// this error should never happen if validation is working
		return nil, types.ErrWildcardNotSupported
	}
	return append(workloadIdentities, newWorkloadIdentities...), nil
}

// tracks a new TP for a given WI, returns the new list of tracked TPs for that WI
func (tm *TrafficPermissionsMapper) trackTrafficPermissionsForCTP(tp *pbresource.ID, ctp *pbresource.ID) []*pbresource.Reference {
	// Update the bimapper entry with a new link
	tpsForWI := append(tm.mapper.LinkRefsForItem(ctp), resource.ReferenceFromReferenceOrID(tp))
	var tpsAsIDsOrRefs []resource.ReferenceOrID
	for _, ref := range tpsForWI {
		tpsAsIDsOrRefs = append(tpsAsIDsOrRefs, ref)
	}
	tm.mapper.TrackItem(ctp, tpsAsIDsOrRefs)
	return tpsForWI
}

func (tm *TrafficPermissionsMapper) UntrackTrafficPermissions(tp *pbresource.ID) {
	// remove associations with workload identities
	tm.mapper.UntrackLink(tp)
	tm.missingMap.untrack(tp)
}

func (tm *TrafficPermissionsMapper) UntrackWorkloadIdentity(ctp *pbresource.ID) {
	// take any associated TPs from bimapper and put them in missingWIMap
	tps := tm.mapper.LinkRefsForItem(ctp)
	for _, t := range tps {
		tm.missingMap.track(resource.IDFromReference(t), ctp.Name)
	}
	// remove from bimapper
	tm.mapper.UntrackItem(ctp)
	// remove from tracked WIs
	delete(tm.workloadIdentityToCTPMap, ctp.Name)
	return
}

func (tm *TrafficPermissionsMapper) TrackCTPForWorkloadIdentity(ctp *pbresource.ID, wi *pbresource.ID) {
	// look for matches in the missingWIMap
	unmappedTPs := tm.missingMap.workloadIdentityNameToTP[wi.Name]
	tpIDs := make([]resource.ReferenceOrID, 0)
	for tp := range unmappedTPs {
		tpIDs = append(tpIDs, tp.ToID())
		tm.missingMap.untrack(tp.ToID())
	}
	tm.workloadIdentityToCTPMap[wi.Name] = ctp
	// insert into bimapper
	tm.mapper.TrackItem(ctp, tpIDs)
}

func (tm *TrafficPermissionsMapper) GetTrafficPermissionsForCTP(ctp *pbresource.ID) []*pbresource.Reference {
	// look for matches in the missingWIMap
	unmappedTPs := tm.missingMap.workloadIdentityNameToTP[ctp.Name]
	tpRefs := make([]*pbresource.Reference, 0)
	for tp := range unmappedTPs {
		tpRefs = append(tpRefs, tp.ToReference())
	}
	// look for mapped matches
	mappedTPs := tm.mapper.LinkRefsForItem(ctp)
	return append(tpRefs, mappedTPs...)
}
