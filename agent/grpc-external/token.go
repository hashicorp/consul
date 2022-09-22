package external

import (
	"context"

	"github.com/hashicorp/consul/agent/structs"
	"google.golang.org/grpc/metadata"
)

const metadataKeyToken = "x-consul-token"
const metadataMinQueryIndex = "min-query-index"
const metadataMaxQueryTime = "max-query-time"
const metadataKeyAllowStale = "allow-stale"
const metadataKeyRequireConsistent = "require-consistent"
const metadataKeyUseCache = "use-cache"
const metadataMaxStaleDuration = "max-stale-duration"
const metadataFilter = "filter"

// QueryOptionsFromContext returns the query options in the gRPC metadata attached to the
// given context.
func QueryOptionsFromContext(ctx context.Context) structs.QueryOptions {
	options := structs.QueryOptions{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return options
	}
	toks, ok := md[metadataKeyToken]
	if ok && len(toks) > 0 {
		options.Token = toks[0]
	}
	return options
}

// ContextWithQueryOptions returns a context with the given query options attached.
func ContextWithQueryOptions(ctx context.Context, options structs.QueryOptions) context.Context {
	return metadata.AppendToOutgoingContext(ctx, metadataKeyToken, options.Token)
}
