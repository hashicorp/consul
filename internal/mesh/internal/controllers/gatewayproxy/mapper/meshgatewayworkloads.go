// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mapper

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// AllMeshGatewayWorkloadsInPartition returns one controller.Request for each Workload
// selected by a MeshGateway in the partition of the Resource.
var AllMeshGatewayWorkloadsInPartition = func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	fetcher := fetcher.New(rt.Client)

	gateways, err := fetcher.FetchMeshGateways(ctx, &pbresource.Tenancy{
		Partition: res.Id.Tenancy.Partition,
	})
	if err != nil {
		return nil, err
	}

	var requests []controller.Request

	for _, gateway := range gateways {
		endpointsID := resource.ReplaceType(pbcatalog.ServiceEndpointsType, gateway.Id)

		endpoints, err := fetcher.FetchServiceEndpoints(ctx, endpointsID)
		if err != nil {
			continue
		}

		if endpoints == nil || endpoints.Data == nil {
			continue
		}

		for _, endpoint := range endpoints.Data.Endpoints {
			requests = append(requests, controller.Request{
				ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, endpoint.TargetRef),
			})
		}
	}

	return requests, nil
}

var APIGatewaysInParentRefs = func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	fetcher := fetcher.New(rt.Client)

	requests := make([]controller.Request, 0)

	route, err := fetcher.FetchTCPRoute(ctx, res.Id)
	if err != nil {
		return nil, err
	}

	if route == nil {
		return nil, nil
	}

	for _, parentRef := range route.Data.GetParentRefs() {
		if !resource.EqualType(parentRef.Ref.Type, pbmesh.APIGatewayType) {
			return nil, fmt.Errorf("parent reference type %s is not supported", parentRef.Ref.Type)
		}

		endpointsID := resource.ReplaceType(pbcatalog.ServiceEndpointsType, resource.IDFromReference(parentRef.Ref))
		endpoints, err := fetcher.FetchServiceEndpoints(ctx, endpointsID)
		if err != nil {
			continue
		}

		if endpoints == nil || endpoints.Data == nil {
			continue
		}

		for _, endpoint := range endpoints.Data.Endpoints {
			requests = append(requests, controller.Request{
				ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, endpoint.TargetRef),
			})
		}
	}

	fmt.Println(requests)
	fmt.Println("WE DID IT")
	fmt.Println()
	fmt.Println()

	return requests, nil
}
