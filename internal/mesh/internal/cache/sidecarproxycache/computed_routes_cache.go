// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxycache

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type ComputedRoutesCache struct {
	mapper *bimapper.Mapper
}

func NewComputedRoutesCache() *ComputedRoutesCache {
	return &ComputedRoutesCache{
		mapper: bimapper.New(pbmesh.ComputedRoutesType, pbcatalog.ServiceType),
	}
}

func (c *ComputedRoutesCache) TrackComputedRoutes(computedRoutes *types.DecodedComputedRoutes) {
	var serviceRefs []resource.ReferenceOrID

	for _, pcr := range computedRoutes.Data.PortedConfigs {
		for _, details := range pcr.Targets {
			serviceRefs = append(serviceRefs, details.BackendRef.Ref)
		}
	}

	c.mapper.TrackItem(computedRoutes.Resource.Id, serviceRefs)
}

func (c *ComputedRoutesCache) UntrackComputedRoutes(computedRoutesID *pbresource.ID) {
	c.mapper.UntrackItem(computedRoutesID)
}

func (c *ComputedRoutesCache) ComputedRoutesByService(id resource.ReferenceOrID) []*pbresource.ID {
	return c.mapper.ItemIDsForLink(id)
}
