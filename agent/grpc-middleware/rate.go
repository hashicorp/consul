package middleware

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/tap"

	"github.com/hashicorp/consul/agent/consul/rate"
)

// ServerRateLimiterMiddleware implements a ServerInHandle function to perform
// RPC rate limiting at the cheapest possible point (before the full request has
// been decoded).
func ServerRateLimiterMiddleware(limiter RateLimiter) tap.ServerInHandle {
	return func(ctx context.Context, info *tap.Info) (context.Context, error) {
		peer, ok := peer.FromContext(ctx)
		if !ok {
			// This should never happen!
			return ctx, status.Error(codes.Internal, "gRPC rate limit middleware unable to read peer")
		}

		err := limiter.Allow(rate.Operation{
			Name:       info.FullMethodName,
			SourceAddr: peer.Addr,
			// TODO: operation type.
		})

		switch {
		case err == nil:
			return ctx, nil
		case errors.Is(err, rate.ErrRetryElsewhere):
			return ctx, status.Error(codes.ResourceExhausted, err.Error())
		default:
			return ctx, status.Error(codes.Unavailable, err.Error())
		}
	}
}

//go:generate mockery --name RateLimiter --inpackage
type RateLimiter interface {
	Allow(rate.Operation) error
}

// NullRateLimiter returns a RateLimiter that allows every operation.
func NullRateLimiter() RateLimiter {
	return nullRateLimiter{}
}

type nullRateLimiter struct{}

func (nullRateLimiter) Allow(rate.Operation) error { return nil }
