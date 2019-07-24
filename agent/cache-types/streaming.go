package cachetype

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/stream"
	"google.golang.org/grpc"
)

type GRPCClient interface {
	Subscribe(ctx context.Context, in *stream.SubscribeRequest, opts ...grpc.CallOption) (stream.Consul_SubscribeClient, error)
}
