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

func (f *Fetcher) FetchMeshGateway(ctx context.Context, id *pbresource.ID) (*types.DecodedMeshGateway, error) {
	dec, err := resource.GetDecodedResource[*pbmesh.MeshGateway](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// We also need to make sure to delete the associated gateway from cache.
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
		// We also need to make sure to delete the associated proxy from cache.
		// TODO f.cache.UntrackProxyStateTemplate(id)
		return nil, nil
	}

	// TODO f.cache.TrackProxyStateTemplate(dec)

	return dec, err
}

func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*types.DecodedWorkload, error) {
	dec, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// We also need to make sure to delete the associated proxy from cache.
		// TODO f.cache.UntrackWorkload(id)
		return nil, nil
	}

	// TODO f.cache.TrackWorkload(dec)

	return dec, err
}
