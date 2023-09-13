// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxycache

import (
	meshapi "github.com/hashicorp/consul/api/mesh/v2beta1"
	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// IdentitiesCache tracks mappings between workload identities and proxy IDs
// that a configuration applies to. It is the responsibility of the controller to
// keep this cache up-to-date.
type IdentitiesCache struct {
	mapper *bimapper.Mapper
}

func NewIdentitiesCache() *IdentitiesCache {
	return &IdentitiesCache{
		mapper: bimapper.New(meshapi.ProxyStateTemplateType, auth.WorkloadIdentityType),
	}
}

func (c *IdentitiesCache) ProxyIDsByWorkloadIdentity(id *pbresource.ID) []*pbresource.ID {
	return c.mapper.ItemIDsForLink(id)
}

func (c *IdentitiesCache) TrackPair(identityID *pbresource.ID, proxyID *pbresource.ID) {
	c.mapper.TrackItem(proxyID, []resource.ReferenceOrID{identityID})
}

// UntrackProxyID removes tracking for the given proxy state template ID.
func (c *IdentitiesCache) UntrackProxyID(proxyID *pbresource.ID) {
	c.mapper.UntrackItem(proxyID)
}
