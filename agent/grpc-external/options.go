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
		return options, nil
	}

	m := map[string]string{}
	for k, v := range md {
		m[k] = v[0]
	}

	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &options,
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return structs.QueryOptions{}, err
	}

	err = decoder.Decode(m)
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
