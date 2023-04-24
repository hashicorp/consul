package external

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const metadataKeyToken = "x-consul-token"

// TokenFromContext returns the ACL token in the gRPC metadata attached to the
// given context.
func TokenFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	toks, ok := md[metadataKeyToken]
	if ok && len(toks) > 0 {
		return toks[0]
	}
	return ""
}

// ContextWithToken returns a context with the given ACL token attached.
func ContextWithToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, metadataKeyToken, token)
}
