package external

import (
	"context"
	"fmt"

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
		return options, fmt.Errorf("could not get metadata from context")
	}

	m := map[string]string{}
	for k, v := range md {
		m[k] = v[0]
	}

	err := mapstructure.Decode(m, &options)
	if err != nil {
		return structs.QueryOptions{}, err
	}

	return options, nil
}

// ContextWithQueryOptions returns a context with the given query options attached.
func ContextWithQueryOptions(ctx context.Context, options structs.QueryOptions) (context.Context, error) {
	md := metadata.MD{}
	m := map[string]interface{}{}
	err := mapstructure.Decode(options, &m)
	if err != nil {
		return nil, err
	}
	for k, v := range m {
		md.Set(k, fmt.Sprintf("%v", v))
	}
	return metadata.NewOutgoingContext(ctx, md), nil
}
