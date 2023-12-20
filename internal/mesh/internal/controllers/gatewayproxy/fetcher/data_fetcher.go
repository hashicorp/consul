// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Fetcher struct {
	client pbresource.ResourceServiceClient
	cache  *cache.Cache
}

func New(client pbresource.ResourceServiceClient, cache *cache.Cache) *Fetcher {
	return &Fetcher{
		client: client,
		cache:  cache,
	}
}

func (f *Fetcher) FetchComputedExportedServices(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedExportedServices, error) {
	dec, err := resource.GetDecodedResource[*pbmulticluster.ComputedExportedServices](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// TODO f.cache.UntrackComputedExportedServices(id)
		return nil, nil
	}

	// TODO f.cache.TrackComputedExportedServices(dec)

	return dec, err
}

func (f *Fetcher) FetchMeshGateway(ctx context.Context, id *pbresource.ID) (*types.DecodedMeshGateway, error) {
	dec, err := resource.GetDecodedResource[*pbmesh.MeshGateway](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// TODO f.cache.UntrackMeshGateway(id)
		return nil, nil
	}

	// TODO f.cache.TrackMeshGateway(dec)

	return dec, err
}

func (f *Fetcher) FetchProxyStateTemplate(ctx context.Context, id *pbresource.ID) (*types.DecodedProxyStateTemplate, error) {
	dec, err := resource.GetDecodedResource[*pbmesh.ProxyStateTemplate](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// TODO f.cache.UntrackProxyStateTemplate(id)
		return nil, nil
	}

	// TODO f.cache.TrackProxyStateTemplate(dec)

	return dec, err
}

func (f *Fetcher) FetchService(ctx context.Context, id *pbresource.ID) (*types.DecodedService, error) {
	dec, err := resource.GetDecodedResource[*pbcatalog.Service](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// TODO f.cache.UntrackService(id)
		return nil, nil
	}

	// TODO f.cache.TrackService(dec)

	return dec, err
}

func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*types.DecodedWorkload, error) {
	dec, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// TODO f.cache.UntrackWorkload(id)
		return nil, nil
	}

	// TODO f.cache.TrackWorkload(dec)

	return dec, err
}
