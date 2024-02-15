// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var boundRefsIndex = indexers.BoundRefsIndex[*pbmesh.ComputedRoutes]("bound-references")

var (
	httpRouteByParentServiceIndex = XRouteByParentRefIndex[*pbmesh.HTTPRoute]("http-route-by-parent-ref")
	grpcRouteByParentServiceIndex = XRouteByParentRefIndex[*pbmesh.GRPCRoute]("grpc-route-by-parent-ref")
	tcpRouteByParentServiceIndex  = XRouteByParentRefIndex[*pbmesh.TCPRoute]("tcp-route-by-parent-ref")

	httpRouteByBackendServiceIndex = XRouteByBackendRefIndex[*pbmesh.HTTPRoute]("http-route-by-backend-ref")
	grpcRouteByBackendServiceIndex = XRouteByBackendRefIndex[*pbmesh.GRPCRoute]("grpc-route-by-backend-ref")
	tcpRouteByBackendServiceIndex  = XRouteByBackendRefIndex[*pbmesh.TCPRoute]("tcp-route-by-backend-ref")
)

// Cache: reverse xRoute => SVC[parent]
func XRouteByParentRefIndex[T types.XRouteData](name string) *index.Index {
	return indexers.RefOrIDIndex[T](name, func(res *resource.DecodedResource[T]) []*pbresource.Reference {
		return parentRefSliceToRefSlice(res.Data.GetParentRefs())
	})
}

func parentRefSliceToRefSlice(parentRefs []*pbmesh.ParentReference) []*pbresource.Reference {
	if parentRefs == nil {
		return nil
	}
	parents := make([]*pbresource.Reference, 0, len(parentRefs))
	for _, parentRef := range parentRefs {
		if parentRef.Ref != nil {
			parents = append(parents, parentRef.Ref)
		}
	}
	return parents
}

// Cache: reverse xRoute => SVC[backend]
func XRouteByBackendRefIndex[T types.XRouteData](name string) *index.Index {
	return indexers.RefOrIDIndex[T](name, func(res *resource.DecodedResource[T]) []*pbresource.Reference {
		return backendRefSliceToRefSlice(res.Data.GetUnderlyingBackendRefs())
	})
}

func backendRefSliceToRefSlice(backendRefs []*pbmesh.BackendReference) []*pbresource.Reference {
	if backendRefs == nil {
		return nil
	}
	refs := make([]*pbresource.Reference, 0, len(backendRefs))
	for _, backendRef := range backendRefs {
		if backendRef.Ref != nil {
			refs = append(refs, backendRef.Ref)
		}
	}
	return refs
}

var computedFailoverByDestRefIndex = indexers.RefOrIDIndex(
	"computed-failover-by-dest-ref",
	func(dec *resource.DecodedResource[*pbcatalog.ComputedFailoverPolicy]) []*pbresource.Reference {
		return dec.Data.GetUnderlyingDestinationRefs()
	},
)
