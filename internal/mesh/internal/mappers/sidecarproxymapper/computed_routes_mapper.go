// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapComputedRoutesToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var computedRoutes pbmesh.ComputedRoutes
	err := res.Data.UnmarshalTo(&computedRoutes)
	if err != nil {
		return nil, err
	}

	// Each Destination gets a single ComputedRoutes.
	serviceID := resource.ReplaceType(catalog.ServiceType, res.Id)
	serviceRef := resource.Reference(serviceID, "")

	var result []controller.Request

	for port := range computedRoutes.PortedConfigs {
		dest, ok := m.destinationsCache.ReadDestination(serviceRef, port)
		if !ok {
			continue // skip
		}

		for rk := range dest.SourceProxies {
			result = append(result, controller.Request{ID: rk.ToID()})
		}
	}

	// todo (ishustava): this is a stub for now until we implement implicit destinations.
	// For tproxy, we generate requests for all proxy states in the cluster.
	// This will generate duplicate events for proxies already added above,
	// however, we expect that the controller runtime will de-dup for us.
	rsp, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Type: types.ProxyStateTemplateType,
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: res.Id.Tenancy.Partition,
			PeerName:  "local",
		},
	})
	if err != nil {
		return nil, err
	}
	for _, r := range rsp.Resources {
		result = append(result, controller.Request{ID: r.Id})
	}

	return result, nil
}
