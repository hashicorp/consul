// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type memoizingLoader struct {
	client pbresource.ResourceServiceClient

	httpRoutes       map[resource.ReferenceKey]*types.DecodedHTTPRoute
	grpcRoutes       map[resource.ReferenceKey]*types.DecodedGRPCRoute
	tcpRoutes        map[resource.ReferenceKey]*types.DecodedTCPRoute
	destPolicies     map[resource.ReferenceKey]*types.DecodedDestinationPolicy
	failoverPolicies map[resource.ReferenceKey]*types.DecodedFailoverPolicy
	services         map[resource.ReferenceKey]*types.DecodedService
}

func newMemoizingLoader(client pbresource.ResourceServiceClient) *memoizingLoader {
	if client == nil {
		panic("client is required")
	}
	return &memoizingLoader{
		client:           client,
		httpRoutes:       make(map[resource.ReferenceKey]*types.DecodedHTTPRoute),
		grpcRoutes:       make(map[resource.ReferenceKey]*types.DecodedGRPCRoute),
		tcpRoutes:        make(map[resource.ReferenceKey]*types.DecodedTCPRoute),
		destPolicies:     make(map[resource.ReferenceKey]*types.DecodedDestinationPolicy),
		failoverPolicies: make(map[resource.ReferenceKey]*types.DecodedFailoverPolicy),
		services:         make(map[resource.ReferenceKey]*types.DecodedService),
	}
}

// TODO: figure out how to code-gen all of these

func (m *memoizingLoader) GetHTTPRoute(ctx context.Context, id *pbresource.ID) (*types.DecodedHTTPRoute, error) {
	if !resource.EqualType(id.Type, types.HTTPRouteType) {
		return nil, fmt.Errorf("expected *mesh.HTTPRoute, not %s", resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := m.httpRoutes[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[pbmesh.HTTPRoute, *pbmesh.HTTPRoute](ctx, m.client, id)
	if err != nil {
		return nil, err
	}

	m.httpRoutes[rk] = dec
	return dec, nil
}

func (m *memoizingLoader) GetGRPCRoute(ctx context.Context, id *pbresource.ID) (*types.DecodedGRPCRoute, error) {
	if !resource.EqualType(id.Type, types.GRPCRouteType) {
		return nil, fmt.Errorf("expected *mesh.GRPCRoute, not %s", resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := m.grpcRoutes[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[pbmesh.GRPCRoute, *pbmesh.GRPCRoute](ctx, m.client, id)
	if err != nil {
		return nil, err
	}

	m.grpcRoutes[rk] = dec
	return dec, nil
}

func (m *memoizingLoader) GetTCPRoute(ctx context.Context, id *pbresource.ID) (*types.DecodedTCPRoute, error) {
	if !resource.EqualType(id.Type, types.TCPRouteType) {
		return nil, fmt.Errorf("expected *mesh.TCPRoute, not %s", resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := m.tcpRoutes[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[pbmesh.TCPRoute, *pbmesh.TCPRoute](ctx, m.client, id)
	if err != nil {
		return nil, err
	}

	m.tcpRoutes[rk] = dec
	return dec, nil
}

func (m *memoizingLoader) GetDestinationPolicy(ctx context.Context, id *pbresource.ID) (*types.DecodedDestinationPolicy, error) {
	if !resource.EqualType(id.Type, types.DestinationPolicyType) {
		return nil, fmt.Errorf("expected *mesh.DestinationPolicy, not %s", resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := m.destPolicies[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[pbmesh.DestinationPolicy, *pbmesh.DestinationPolicy](ctx, m.client, id)
	if err != nil {
		return nil, err
	}

	m.destPolicies[rk] = dec
	return dec, nil
}

func (m *memoizingLoader) GetFailoverPolicy(ctx context.Context, id *pbresource.ID) (*types.DecodedFailoverPolicy, error) {
	if !resource.EqualType(id.Type, catalog.FailoverPolicyType) {
		return nil, fmt.Errorf("expected *catalog.FailoverPolicy, not %s", resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := m.failoverPolicies[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](ctx, m.client, id)
	if err != nil {
		return nil, err
	}

	m.failoverPolicies[rk] = dec
	return dec, nil
}

func (m *memoizingLoader) GetService(ctx context.Context, id *pbresource.ID) (*types.DecodedService, error) {
	if !resource.EqualType(id.Type, catalog.ServiceType) {
		return nil, fmt.Errorf("expected *catalog.Service, not %s", resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := m.services[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[pbcatalog.Service, *pbcatalog.Service](ctx, m.client, id)
	if err != nil {
		return nil, err
	}

	m.services[rk] = dec
	return dec, nil
}
