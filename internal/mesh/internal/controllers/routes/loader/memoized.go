// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type memoizingLoader struct {
	client pbresource.ResourceServiceClient

	mapHTTPRoute         map[resource.ReferenceKey]*types.DecodedHTTPRoute
	mapGRPCRoute         map[resource.ReferenceKey]*types.DecodedGRPCRoute
	mapTCPRoute          map[resource.ReferenceKey]*types.DecodedTCPRoute
	mapDestinationPolicy map[resource.ReferenceKey]*types.DecodedDestinationPolicy
	mapFailoverPolicy    map[resource.ReferenceKey]*types.DecodedFailoverPolicy
	mapService           map[resource.ReferenceKey]*types.DecodedService
}

func newMemoizingLoader(client pbresource.ResourceServiceClient) *memoizingLoader {
	if client == nil {
		panic("client is required")
	}
	return &memoizingLoader{
		client:               client,
		mapHTTPRoute:         make(map[resource.ReferenceKey]*types.DecodedHTTPRoute),
		mapGRPCRoute:         make(map[resource.ReferenceKey]*types.DecodedGRPCRoute),
		mapTCPRoute:          make(map[resource.ReferenceKey]*types.DecodedTCPRoute),
		mapDestinationPolicy: make(map[resource.ReferenceKey]*types.DecodedDestinationPolicy),
		mapFailoverPolicy:    make(map[resource.ReferenceKey]*types.DecodedFailoverPolicy),
		mapService:           make(map[resource.ReferenceKey]*types.DecodedService),
	}
}

func (m *memoizingLoader) GetHTTPRoute(ctx context.Context, id *pbresource.ID) (*types.DecodedHTTPRoute, error) {
	return getOrCacheResource[*pbmesh.HTTPRoute](ctx, m.client, m.mapHTTPRoute, pbmesh.HTTPRouteType, id)
}

func (m *memoizingLoader) GetGRPCRoute(ctx context.Context, id *pbresource.ID) (*types.DecodedGRPCRoute, error) {
	return getOrCacheResource[*pbmesh.GRPCRoute](ctx, m.client, m.mapGRPCRoute, pbmesh.GRPCRouteType, id)
}

func (m *memoizingLoader) GetTCPRoute(ctx context.Context, id *pbresource.ID) (*types.DecodedTCPRoute, error) {
	return getOrCacheResource[*pbmesh.TCPRoute](ctx, m.client, m.mapTCPRoute, pbmesh.TCPRouteType, id)
}

func (m *memoizingLoader) GetDestinationPolicy(ctx context.Context, id *pbresource.ID) (*types.DecodedDestinationPolicy, error) {
	return getOrCacheResource[*pbmesh.DestinationPolicy](ctx, m.client, m.mapDestinationPolicy, pbmesh.DestinationPolicyType, id)
}

func (m *memoizingLoader) GetFailoverPolicy(ctx context.Context, id *pbresource.ID) (*types.DecodedFailoverPolicy, error) {
	return getOrCacheResource[*pbcatalog.FailoverPolicy](ctx, m.client, m.mapFailoverPolicy, pbcatalog.FailoverPolicyType, id)
}

func (m *memoizingLoader) GetService(ctx context.Context, id *pbresource.ID) (*types.DecodedService, error) {
	return getOrCacheResource[*pbcatalog.Service](ctx, m.client, m.mapService, pbcatalog.ServiceType, id)
}

func getOrCacheResource[T proto.Message](
	ctx context.Context,
	client pbresource.ResourceServiceClient,
	cache map[resource.ReferenceKey]*resource.DecodedResource[T],
	typ *pbresource.Type,
	id *pbresource.ID,
) (*resource.DecodedResource[T], error) {
	if !resource.EqualType(id.Type, typ) {
		return nil, fmt.Errorf("expected %s not %s", resource.TypeToString(typ), resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := cache[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[T](ctx, client, id)
	if err != nil {
		return nil, err
	}

	cache[rk] = dec
	return dec, nil
}
