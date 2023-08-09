package wrappedtypes

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

//go:generate ../../../tools/wrapped-protobuf-types/generate.sh http_route.gen.go pbmesh.HTTPRoute
//go:generate ../../../tools/wrapped-protobuf-types/generate.sh grpc_route.gen.go pbmesh.GRPCRoute
//go:generate ../../../tools/wrapped-protobuf-types/generate.sh tcp_route.gen.go pbmesh.TCPRoute
//go:generate ../../../tools/wrapped-protobuf-types/generate.sh dest_policy.gen.go pbmesh.DestinationPolicy
//go:generate ../../../tools/wrapped-protobuf-types/generate.sh computed_routes.gen.go pbmesh.ComputedRoutes
//go:generate ../../../tools/wrapped-protobuf-types/generate.sh failover_policy.gen.go pbcatalog.FailoverPolicy
//go:generate ../../../tools/wrapped-protobuf-types/generate.sh service.gen.go pbcatalog.Service

type Wrapped interface {
	GetResource() *pbresource.Resource
}

type WrappedRoute interface {
	Wrapped
	types.XRouteData
}
