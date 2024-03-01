// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Fetcher struct {
	client pbresource.ResourceServiceClient
}

func New(client pbresource.ResourceServiceClient) *Fetcher {
	return &Fetcher{
		client: client,
	}
}

// FetchAllTCPRoutes fetches all the tcp routes.
// TODO: in the future this will not be necessary as we'll use the computed gateway routes.
func (f *Fetcher) FetchAllTCPRoutes(ctx context.Context, tenancy *pbresource.Tenancy) ([]*types.DecodedTCPRoute, error) {
	dec, err := resource.ListDecodedResource[*pbmesh.TCPRoute](ctx, f.client, &pbresource.ListRequest{
		Type:    pbmesh.TCPRouteType,
		Tenancy: tenancy,
	})
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchAPIGateway fetches a service resource from the resource service.
// This will panic if the type field in the ID argument is not a APIGateway type.
func (f *Fetcher) FetchAPIGateway(ctx context.Context, id *pbresource.ID) (*types.DecodedAPIGateway, error) {
	assertResourceType(pbmesh.APIGatewayType, id.Type)

	dec, err := resource.GetDecodedResource[*pbmesh.APIGateway](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchComputedExportedServices fetches a service resource from the resource service.
// This will panic if the type field in the ID argument is not a ComputedExportedServices type.
func (f *Fetcher) FetchComputedExportedServices(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedExportedServices, error) {
	assertResourceType(pbmulticluster.ComputedExportedServicesType, id.Type)

	dec, err := resource.GetDecodedResource[*pbmulticluster.ComputedExportedServices](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchMeshGateway fetches a service resource from the resource service.
// This will panic if the type field in the ID argument is not a MeshGateway type.
func (f *Fetcher) FetchMeshGateway(ctx context.Context, id *pbresource.ID) (*types.DecodedMeshGateway, error) {
	assertResourceType(pbmesh.MeshGatewayType, id.Type)

	dec, err := resource.GetDecodedResource[*pbmesh.MeshGateway](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchMeshGateways fetches all MeshGateway resources known to the local server.
func (f *Fetcher) FetchMeshGateways(ctx context.Context, tenancy *pbresource.Tenancy) ([]*types.DecodedMeshGateway, error) {
	dec, err := resource.ListDecodedResource[*pbmesh.MeshGateway](ctx, f.client, &pbresource.ListRequest{
		Type:    pbmesh.MeshGatewayType,
		Tenancy: tenancy,
	})
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchProxyStateTemplate fetches a service resource from the resource service.
// This will panic if the type field in the ID argument is not a ProxyStateTemplate type.
func (f *Fetcher) FetchProxyStateTemplate(ctx context.Context, id *pbresource.ID) (*types.DecodedProxyStateTemplate, error) {
	assertResourceType(pbmesh.ProxyStateTemplateType, id.Type)

	dec, err := resource.GetDecodedResource[*pbmesh.ProxyStateTemplate](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchService fetches a service resource from the resource service.
// This will panic if the type field in the ID argument is not a Service type.
func (f *Fetcher) FetchService(ctx context.Context, id *pbresource.ID) (*types.DecodedService, error) {
	assertResourceType(pbcatalog.ServiceType, id.Type)

	dec, err := resource.GetDecodedResource[*pbcatalog.Service](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

func (f *Fetcher) FetchServiceEndpoints(ctx context.Context, id *pbresource.ID) (*types.DecodedServiceEndpoints, error) {
	assertResourceType(pbcatalog.ServiceEndpointsType, id.Type)

	dec, err := resource.GetDecodedResource[*pbcatalog.ServiceEndpoints](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchTCPRoute fetches all the tcp routes.
// TODO: in the future this will not be necessary as we'll use the computed gateway routes.
func (f *Fetcher) FetchTCPRoute(ctx context.Context, id *pbresource.ID) (*types.DecodedTCPRoute, error) {
	assertResourceType(pbmesh.TCPRouteType, id.Type)

	dec, err := resource.GetDecodedResource[*pbmesh.TCPRoute](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// FetchWorkload fetches a service resource from the resource service.
// This will panic if the type field in the ID argument is not a Workload type.
func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*types.DecodedWorkload, error) {
	assertResourceType(pbcatalog.WorkloadType, id.Type)

	dec, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// this is a helper function to ensure that the resource type we are querying for is the type we expect
func assertResourceType(expected, actual *pbresource.Type) {
	if !proto.Equal(expected, actual) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("expected a query for a type of %q, you provided a type of %q", expected, actual))
	}
}
