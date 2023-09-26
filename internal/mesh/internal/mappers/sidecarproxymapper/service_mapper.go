// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapServiceToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	serviceRef := resource.Reference(res.Id, "")

	ids, err := m.mapServiceThroughDestinationsToProxyStateTemplates(ctx, rt, serviceRef)
	if err != nil {
		return nil, err
	}

	return controller.MakeRequests(pbmesh.ProxyStateTemplateType, ids), nil
}

// mapServiceThroughDestinationsToProxyStateTemplates takes an explicit
// Service and traverses back through Destinations to Workloads to
// ProxyStateTemplates.
//
// This is in a separate function so it can be chained for more complicated
// relationships.
func (m *Mapper) mapServiceThroughDestinationsToProxyStateTemplates(
	ctx context.Context,
	rt controller.Runtime,
	serviceRef *pbresource.Reference,
) ([]*pbresource.ID, error) {

	// The relationship is:
	//
	// - PST (replace type) Workload
	// - Workload (selected by) Upstreams
	// - Upstream (contains) Service
	//
	// When we wake up for Service we should:
	//
	// - look for Service in all Destinations(upstreams)
	// - follow selectors backwards to Workloads
	// - rewrite types to PST

	var pstIDs []*pbresource.ID

	destinations := m.destinationsCache.ReadDestinationsByServiceAllPorts(serviceRef)
	for _, destination := range destinations {
		for refKey := range destination.SourceProxies {
			pstIDs = append(pstIDs, refKey.ToID())
		}
	}

	// TODO(v2): remove this after we can do proper performant implicit upstream determination
	//
	// TODO(rb): shouldn't this instead list all Workloads that have a mesh port?
	allIDs, err := m.listAllProxyStateTemplatesTemporarily(ctx, rt, serviceRef.Tenancy)
	if err != nil {
		return nil, err
	}

	pstIDs = append(pstIDs, allIDs...)

	return pstIDs, nil
}

func (m *Mapper) listAllProxyStateTemplatesTemporarily(ctx context.Context, rt controller.Runtime, tenancy *pbresource.Tenancy) ([]*pbresource.ID, error) {
	// todo (ishustava): this is a stub for now until we implement implicit destinations.
	// For tproxy, we generate requests for all proxy states in the cluster.
	// This will generate duplicate events for proxies already added above,
	// however, we expect that the controller runtime will de-dup for us.
	rsp, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Type: pbmesh.ProxyStateTemplateType,
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: tenancy.Partition,
			PeerName:  tenancy.PeerName,
		},
	})
	if err != nil {
		return nil, err
	}

	result := make([]*pbresource.ID, 0, len(rsp.Resources))
	for _, r := range rsp.Resources {
		result = append(result, r.Id)
	}
	return result, nil
}
