// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxycache

import (
	"fmt"
	"sync"

	auth "github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// IdentitiesCache tracks mappings between workload identities and proxy IDs
// that a configuration applies to. It is the responsibility of the controller to
// keep this cache up-to-date.
type IdentitiesCache struct {
	lock sync.RWMutex

	identityToProxyIDs map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}
	proxyIDToIdentity  map[resource.ReferenceKey]resource.ReferenceKey
}

func NewIdentitiesCache() *IdentitiesCache {
	return &IdentitiesCache{
		identityToProxyIDs: make(map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}),
		proxyIDToIdentity:  make(map[resource.ReferenceKey]resource.ReferenceKey),
	}
}

func (c *IdentitiesCache) ProxyIDsByWorkloadIdentity(id *pbresource.ID) []*pbresource.ID {
	checkIdentityType(id)

	c.lock.RLock()
	defer c.lock.RUnlock()

	pids, ok := c.identityToProxyIDs[resource.NewReferenceKey(id)]
	if !ok {
		return nil
	}

	var out []*pbresource.ID

	for pid := range pids {
		out = append(out, pid.ToID())
	}

	return out
}

func (c *IdentitiesCache) TrackPair(identityID *pbresource.ID, proxyID *pbresource.ID) {
	checkProxyType(proxyID)
	checkIdentityType(identityID)

	c.lock.Lock()
	defer c.lock.Unlock()

	id := resource.NewReferenceKey(identityID)
	pid := resource.NewReferenceKey(proxyID)

	if previousID, ok := c.proxyIDToIdentity[pid]; ok && id != previousID {
		c.removeProxyIDFromIdentityLocked(pid, previousID)
	}

	if _, ok := c.identityToProxyIDs[id]; !ok {
		c.identityToProxyIDs[id] = make(map[resource.ReferenceKey]struct{})
	}

	c.identityToProxyIDs[id][pid] = struct{}{}
	c.proxyIDToIdentity[pid] = id
}

// UntrackProxyID removes tracking for the given proxy state template ID.
func (c *IdentitiesCache) UntrackProxyID(proxyID *pbresource.ID) {
	checkProxyType(proxyID)

	c.lock.Lock()
	defer c.lock.Unlock()

	pid := resource.NewReferenceKey(proxyID)
	id := c.proxyIDToIdentity[pid]

	delete(c.proxyIDToIdentity, pid)
	c.removeProxyIDFromIdentityLocked(pid, id)
}

func (c *IdentitiesCache) removeProxyIDFromIdentityLocked(pid, id resource.ReferenceKey) {
	if pids, ok := c.identityToProxyIDs[id]; ok {
		delete(pids, pid)
		if len(pids) == 0 {
			delete(c.identityToProxyIDs, id)
		}
	}
}

func checkIdentityType(id *pbresource.ID) {
	if !resource.EqualType(id.Type, auth.WorkloadIdentityType) {
		panic(fmt.Sprintf("expected type %q got %q",
			resource.TypeToString(auth.WorkloadIdentityType),
			resource.TypeToString(id.Type),
		))
	}
}

func checkProxyType(id *pbresource.ID) {
	if !resource.EqualType(id.Type, types.ProxyStateTemplateType) {
		panic(fmt.Sprintf("expected type %q got %q",
			resource.TypeToString(types.ProxyStateTemplateType),
			resource.TypeToString(id.Type),
		))
	}
}
