package external

import (
	"context"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc/metadata"
)

// QueryOptionsFromContext returns the query options in the gRPC metadata attached to the
// given context.
func QueryOptionsFromContext(ctx context.Context) (structs.QueryOptions, error) {
	options := structs.QueryOptions{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return options, nil
	}
	err := mapstructure.Decode(md, options)
	if err != nil {
		return structs.QueryOptions{}, err
	}
	// toks, ok := md[metadataKeyToken]
	// if ok && len(toks) > 0 {
	// 	options.Token = toks[0]
	// }
	return options, nil
}

// ContextWithQueryOptions returns a context with the given query options attached.
func ContextWithQueryOptions(ctx context.Context, options structs.QueryOptions) (context.Context, error) {
	md := metadata.MD{}
	err := mapstructure.Decode(options, md)
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, md), nil
}
