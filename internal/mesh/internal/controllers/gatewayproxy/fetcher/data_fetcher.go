// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
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
	if !proto.Equal(id.Type, pbmesh.MeshGatewayType) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("FetchMeshGateway expected a query for a type of %q, you provided a type of %q", pbmesh.MeshGatewayType, id.Type))
	}

	dec, err := resource.GetDecodedResource[*pbmesh.MeshGateway](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		return nil, nil
	}

	return dec, nil
}

func (f *Fetcher) FetchProxyStateTemplate(ctx context.Context, id *pbresource.ID) (*types.DecodedProxyStateTemplate, error) {
	if !proto.Equal(id.Type, pbmesh.ProxyStateTemplateType) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("FetchProxyStateTemplate expected a query for a type of %q, you provided a type of %q", pbmesh.ProxyStateTemplateType, id.Type))
	}

	dec, err := resource.GetDecodedResource[*pbmesh.ProxyStateTemplate](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		return nil, nil
	}

	return dec, nil
}

func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*types.DecodedWorkload, error) {
	if !proto.Equal(id.Type, pbcatalog.WorkloadType) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("FetchWorkload expected a query for a type of %q, you provided a type of %q", pbcatalog.WorkloadType, id.Type))
	}

	dec, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		return nil, nil
	}

	return dec, nil
}

func (f *Fetcher) FetchExportedServices(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedExportedServices, error) {
	if !proto.Equal(id.Type, pbmulticluster.ComputedExportedServicesType) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("FetchExportedServices expected a query for a type of %q, you provided a type of %q", pbmulticluster.ComputedExportedServicesType, id.Type))
	}
	dec, err := resource.GetDecodedResource[*pbmulticluster.ComputedExportedServices](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		return nil, nil
	}

	return dec, nil
}

func (f *Fetcher) FetchService(ctx context.Context, id *pbresource.ID) (*types.DecodedService, error) {
	if !proto.Equal(id.Type, pbcatalog.ServiceType) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("FetchService expected a query for a type of %q, you provided a type of %q", pbcatalog.ServiceType, id.Type))
	}

	dec, err := resource.GetDecodedResource[*pbcatalog.Service](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		return nil, nil
	}

	return dec, nil
}
