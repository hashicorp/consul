// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissionsmapper

import (
	"context"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/radix"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TODO: methods to map to NamespaceTrafficPermissionsMapper
type PartitionTrafficPermissionsMapper struct {
	lock                       sync.Mutex
	partitionToNamespaceMapper map[string]NamespaceTrafficPermissionsMapper
}

// TODO: methods to map to WorkloadIdentityMapper
type NamespaceTrafficPermissionsMapper struct {
	lock                       sync.Mutex
	partitionToNamespaceMapper map[string]WorkloadIdentityMapper
}

type radixNode struct {
	controllerReqs []controller.Request
	tps            []resource.ReferenceKey
}

// TODO: tie this to a namepspace
type prefixMapper struct {
	lock sync.Mutex

	// workloadIdentityPrefixes is used to find workload identities from prefixes
	workloadIdentityPrefixes *radix.Tree[radixNode]

	//TODO: make generic version of bimapper for this
	prefixTPMap map[string]map[resource.ReferenceKey]bool
	tpPrefixMap map[resource.ReferenceKey]map[string]bool
}

type missingWorkloadIdentityMap struct {
	// This holds traffic permissions with explicit destinations that are missing a workload identity
	// TODO: decide if we should require WorkloadIdentity to already exist on TP validation
	// Either way, this will still be needed if we do not want to clean up TrafficPermissions after
	// WorkloadIdentity deletion.
	// TODO: We wouldn't need this if we can make a generic bimapper that doesn't clean up dead-end links.
	lock         sync.Mutex
	missingWIMap map[string]map[resource.ReferenceKey]bool // indexes on the name of the missing Workload Identity
}

type WorkloadIdentityMapper struct {
	mapper       *bimapper.Mapper
	prefixMapper *prefixMapper
	missingMap   *missingWorkloadIdentityMap
}

func New() *WorkloadIdentityMapper {
	return &WorkloadIdentityMapper{
		mapper: bimapper.New(types.WorkloadIdentityType, types.TrafficPermissionsType),
		prefixMapper: &prefixMapper{
			lock:                     sync.Mutex{},
			workloadIdentityPrefixes: radix.New[radixNode](),
			prefixTPMap:              make(map[string]map[resource.ReferenceKey]bool),
			tpPrefixMap:              make(map[resource.ReferenceKey]map[string]bool),
		},
		missingMap: &missingWorkloadIdentityMap{
			lock:         sync.Mutex{},
			missingWIMap: make(map[string]map[resource.ReferenceKey]bool),
		},
	}
}

//  WorkloadIdentityPrefixMapper functions

func (pm *prefixMapper) trackTrafficPermissionsForPrefix(tp *pbresource.ID, p string) {
	refKey := resource.NewReferenceKey(tp)
	pm.lock.Lock()
	defer pm.lock.Unlock()
	// insert tp into radix node
	node, exists := pm.workloadIdentityPrefixes.Get(p)
	if exists {
		pm.workloadIdentityPrefixes.Insert(p, radixNode{nil, append(node.tps, refKey)})
	} else {
		pm.workloadIdentityPrefixes.Insert(p, radixNode{nil, []resource.ReferenceKey{refKey}})
	}
	pm.tpPrefixMap[refKey][p] = true
	pm.prefixTPMap[p][refKey] = true
}

func (pm *prefixMapper) untrackTrafficPermissionForPrefixes(tp *pbresource.ID) {
	refKey := resource.NewReferenceKey(tp)
	pm.lock.Lock()
	defer pm.lock.Unlock()
	prefixesForTP, present := pm.tpPrefixMap[refKey]
	if !present {
		// traffic permission is not associated with any prefixes
		return
	}
	// update the prefixTPMap
	for prefix, _ := range prefixesForTP {
		// get TPs with that prefix
		tps := pm.prefixTPMap[prefix]
		_, ok := tps[refKey]
		if !ok {
			panic("traffic permission prefix map data inconsistency")
		}
		if len(tps) == 1 && tps[refKey] {
			// if this was the only TP, remove the prefix from the map
			delete(pm.prefixTPMap, prefix)
		} else {
			// remove this TP from the list
			delete(tps, refKey)
			pm.prefixTPMap[prefix] = tps
		}
	}
	// remove the TP from the tpPrefixMap
	delete(pm.tpPrefixMap, refKey)
}

func (pm *prefixMapper) trackWorkloadIdentity(wi *pbresource.ID) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	// enter leaf node into radix
	pm.workloadIdentityPrefixes.Insert(wi.Name, radixNode{[]controller.Request{{ID: wi}}, nil})
}

func (pm *prefixMapper) untrackWorkloadIdentity(wi *pbresource.ID) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.workloadIdentityPrefixes.Delete(wi.Name)
}

func (pm *prefixMapper) workloadIdentitiesForTrackedTP(tp *pbresource.ID) []controller.Request {
	refKey := resource.NewReferenceKey(tp)
	pm.lock.Lock()
	defer pm.lock.Unlock()
	// get prefixes
	prefixesForTP, present := pm.tpPrefixMap[refKey]
	if !present {
		// traffic permission is not associated with any prefixes
		return []controller.Request{}
	}
	//  get the workload identities for each prefix
	var wis []controller.Request
	for prefix, _ := range prefixesForTP {
		pm.workloadIdentityPrefixes.WalkPrefix(prefix, func(path string, node radixNode) bool {
			wis = append(wis, node.controllerReqs...)
			return false
		})
	}
	return wis
}

func (pm *prefixMapper) trafficPermissionsFromAppliedPrefixes(wiName string) []*pbresource.ID {
	var tps []*pbresource.ID
	pm.workloadIdentityPrefixes.WalkPath(wiName, func(path string, node radixNode) bool {
		tpsAtPref := node.tps
		for _, t := range tpsAtPref {
			tps = append(tps, t.ToID())
		}
		return false
	})
	return tps
}

func (pm *prefixMapper) checkPrefix(p string) bool {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	_, ok := pm.prefixTPMap[p]
	return ok
}

func (m *missingWorkloadIdentityMap) trackMissingWorkloadIdentity(wiName string, refKey resource.ReferenceKey) {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok := m.missingWIMap[wiName]
	if !ok {
		m.missingWIMap[wiName] = make(map[resource.ReferenceKey]bool)
	}
	m.missingWIMap[wiName][refKey] = true
}

func (m *missingWorkloadIdentityMap) Get(wiName string) map[resource.ReferenceKey]bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.missingWIMap[wiName]
}

func (m *missingWorkloadIdentityMap) Delete(wiName string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.missingWIMap, wiName)
}

//  WorkloadIdentityMapper functions

func (wm *WorkloadIdentityMapper) MapTrafficPermissions(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	rt.Logger.Info("traffic permission request:", "#id", res.Id, "#data", res.Data)
	var workloadIdentities []controller.Request
	// get already associated WorkloadIdentities/CTPs for reconcile requests:
	// 1. Get by direct association
	associatedWIs := wm.mapper.ItemIDsForLink(res.Id)
	for _, w := range associatedWIs {
		workloadIdentities = append(workloadIdentities, controller.Request{ID: w})
	}
	// 2. Get by prefix
	workloadIdentities = append(workloadIdentities, wm.prefixMapper.workloadIdentitiesForTrackedTP(res.Id)...)

	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: res.Id})
	switch {
	case status.Code(err) == codes.NotFound:
		// TP has been deleted so we should untrack it and only submit reconcile requests for previously associated WIs
		wm.UntrackTrafficPermissions(res.Id)
		return workloadIdentities, nil
	case err != nil:
		return nil, err
	}
	var tp pbauth.TrafficPermissions
	err = rsp.Resource.Data.UnmarshalTo(&tp)
	if err != nil {
		return nil, resource.NewErrDataParse(&tp, err)
	}

	var newWorkloadIdentities []controller.Request
	// get new CTP associations based on destination
	if tp.Destination == nil {
		return nil, types.ErrWildcardNotSupported
	}
	newWorkloadIdentityName := tp.Destination.IdentityName
	newPrefix := tp.Destination.IdentityPrefix
	if len(newWorkloadIdentityName) > 0 {
		// Does the identity exist?
		newWI, err := LookupWorkloadIdentityByName(ctx, rt, res.Id, newWorkloadIdentityName)
		if err != nil {
			return nil, err
		}
		if newWI != nil {
			wm.trackTrafficPermissionsForWI(res.Id, newWI.Id)
			newWorkloadIdentities = append(newWorkloadIdentities, controller.Request{ID: newWI.Id})
		} else {
			// if not add to missingWIMap
			wm.missingMap.trackMissingWorkloadIdentity(newWorkloadIdentityName, resource.NewReferenceKey(res.Id))
		}
	} else if len(newPrefix) > 0 {
		// Does the prefix exist?
		if wm.prefixMapper.checkPrefix(newPrefix) {
			wm.prefixMapper.workloadIdentityPrefixes.WalkPrefix(newPrefix, func(path string, node radixNode) bool {
				newWorkloadIdentities = append(newWorkloadIdentities, node.controllerReqs...)
				return false
			})
		}
		wm.prefixMapper.trackTrafficPermissionsForPrefix(res.Id, newPrefix)
	}
	// TODO: dedup old and new associated workload identities, to avoid unnecessary reconciles
	return append(workloadIdentities, newWorkloadIdentities...), nil
}

func (wm *WorkloadIdentityMapper) trackTrafficPermissionsForWI(tp *pbresource.ID, wi *pbresource.ID) {
	// Update the bimapper entry with a new link
	tpRef := &pbresource.Reference{
		Type:    types.TrafficPermissionsType,
		Tenancy: tp.Tenancy,
		Name:    tp.Name,
	}
	tpsForWI := append(wm.mapper.LinkRefsForItem(wi), tpRef)
	var tpsAsIDsOrRefs []resource.ReferenceOrID
	for _, ref := range tpsForWI {
		tpsAsIDsOrRefs = append(tpsAsIDsOrRefs, ref)
	}
	wm.mapper.TrackItem(wi, tpsAsIDsOrRefs)
}

func (wm *WorkloadIdentityMapper) UntrackTrafficPermissions(tp *pbresource.ID) {
	// remove associations with prefixes and workload identities
	// 1. prefixes
	wm.prefixMapper.untrackTrafficPermissionForPrefixes(tp)
	// 2. workload identities
	wm.mapper.UntrackLink(tp)
}

func (wm *WorkloadIdentityMapper) UntrackWorkloadIdentity(wi *pbresource.ID) {
	// take any associated TPs from bimapper and put them in missingWIMap
	tps := wm.mapper.LinkRefsForItem(wi)
	for _, t := range tps {
		wm.missingMap.trackMissingWorkloadIdentity(wi.Name, resource.NewReferenceKey(t))
	}
	// remove from bimapper
	wm.mapper.UntrackItem(wi)
	// remove from prefix tracker
	wm.prefixMapper.untrackWorkloadIdentity(wi)
	return
}

func (wm *WorkloadIdentityMapper) TrackWorkloadIdentity(wi *pbresource.ID) {
	// insert into prefix tracker
	wm.prefixMapper.trackWorkloadIdentity(wi)
	// look for matches in the missingWIMap
	unmappedTPs := wm.missingMap.Get(wi.Name)
	tpIDs := make([]resource.ReferenceOrID, 0)
	for tp := range unmappedTPs {
		tpIDs = append(tpIDs, tp.ToID())
	}
	// insert into bimapper
	wm.mapper.TrackItem(wi, tpIDs)
	// remove name from missingWIMap
	wm.missingMap.Delete(wi.Name)
}

// computeNewTrafficPermissions will use all associated Traffic Permissions to create new Computed Traffic Permissions data
func (wm *WorkloadIdentityMapper) ComputeNewTrafficPermissions(ctx context.Context, rt controller.Runtime, workloadIdentity *pbresource.ID) (*pbauth.ComputedTrafficPermissions, error) {
	// Part 1: Get all TPs that apply to workload identity
	// explicit permissions
	var allTrafficPermisisons []pbauth.TrafficPermissions
	// Get already associated WorkloadIdentities/CTPs for reconcile requests:
	// Get by direct association
	explicitTPs := wm.mapper.LinkIDsForItem(workloadIdentity)
	// Get by prefix
	prefixTPs := wm.prefixMapper.trafficPermissionsFromAppliedPrefixes(workloadIdentity.Name)
	for _, tp := range append(explicitTPs, prefixTPs...) {
		rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: tp})
		if err != nil {
			rt.Logger.Error("error reading traffic permissions resource for computation", "error", err)
			return nil, err
		}
		var tp pbauth.TrafficPermissions
		err = rsp.Resource.Data.UnmarshalTo(&tp)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to parse traffic permissions data")
		}
		allTrafficPermisisons = append(allTrafficPermisisons, tp)
	}
	// Part 2: For all TPs affecting WI, aggregate Allow and Deny permissions
	ap := make([]*pbauth.Permission, 0)
	dp := make([]*pbauth.Permission, 0)
	for _, t := range allTrafficPermisisons {
		if t.Action == pbauth.Action_ACTION_ALLOW {
			ap = append(ap, t.Permissions...)
		} else {
			dp = append(dp, t.Permissions...)
		}
	}
	return &pbauth.ComputedTrafficPermissions{AllowPermissions: ap, DenyPermissions: dp}, nil
}

// LookupWorkloadIdentityByName finds a workload identity with a specified name in the same tenancy as
// the provided resource. If no workload identity is found, it returns nil.
func LookupWorkloadIdentityByName(ctx context.Context, rt controller.Runtime, r *pbresource.ID, name string) (*pbresource.Resource, error) {
	wi := &pbresource.ID{
		Type:    types.WorkloadIdentityType,
		Tenancy: r.Tenancy,
		Name:    name,
	}
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: wi})
	switch {
	case status.Code(err) == codes.NotFound:
		rt.Logger.Trace("no WorkloadIdentity found for resource", "resource-type", r.Type, "resource-name", r.Name, "workload-identity-name", name)
		return nil, nil
	case err != nil:
		rt.Logger.Error("error retrieving Workload Identity for TrafficPermission", "error", err)
		return nil, err
	}
	activeWI := rsp.Resource
	rt.Logger.Trace("Got active WorkloadIdentity associated with resource", "resource-type", r.Type, "resource-name", r.Name, "workload-identity-name", activeWI.Id.Name)
	return activeWI, nil
}
