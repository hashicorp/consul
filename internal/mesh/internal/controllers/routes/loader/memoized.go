// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

//go:generate ./gen_memoizer_funcs.sh http_route.gen.go pbmesh.HTTPRoute
//go:generate ./gen_memoizer_funcs.sh grpc_route.gen.go pbmesh.GRPCRoute
//go:generate ./gen_memoizer_funcs.sh tcp_route.gen.go pbmesh.TCPRoute
//go:generate ./gen_memoizer_funcs.sh dest_policy.gen.go pbmesh.DestinationPolicy
//go:generate ./gen_memoizer_funcs.sh failover_policy.gen.go pbcatalog.FailoverPolicy
//go:generate ./gen_memoizer_funcs.sh service.gen.go pbcatalog.Service

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
