// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxycache

import (
	catalogapi "github.com/hashicorp/consul/api/catalog/v2beta1"
	meshapi "github.com/hashicorp/consul/api/mesh/v2beta1"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type ComputedRoutesCache struct {
	mapper *bimapper.Mapper
}

func NewComputedRoutesCache() *ComputedRoutesCache {
	return &ComputedRoutesCache{
		mapper: bimapper.New(meshapi.ComputedRoutesType, catalogapi.ServiceType),
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
