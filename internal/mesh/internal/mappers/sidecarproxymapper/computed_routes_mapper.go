// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapComputedRoutesToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	computedRoutes, err := resource.Decode[*pbmesh.ComputedRoutes](res)
	if err != nil {
		return nil, err
	}

	reqs, err := m.mapComputedRoutesToProxyStateTemplate(ctx, rt, res.Id)
	if err != nil {
		return nil, err
	}

	m.computedRoutesCache.TrackComputedRoutes(computedRoutes)

	return reqs, nil
}

func (m *Mapper) mapComputedRoutesToProxyStateTemplate(ctx context.Context, rt controller.Runtime, computedRoutesID *pbresource.ID) ([]controller.Request, error) {
	// Each Destination gets a single ComputedRoutes.
	serviceID := resource.ReplaceType(pbcatalog.ServiceType, computedRoutesID)
	serviceRef := resource.Reference(serviceID, "")

	ids, err := m.mapServiceThroughDestinationsToProxyStateTemplates(ctx, rt, serviceRef)
	if err != nil {
		return nil, err
	}

	return controller.MakeRequests(pbmesh.ProxyStateTemplateType, ids), nil
}
