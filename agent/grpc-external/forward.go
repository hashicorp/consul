package external

import (
	"context"

	"google.golang.org/grpc/metadata"
)

func ForwardMetadataContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	return metadata.NewOutgoingContext(ctx, md)
}
