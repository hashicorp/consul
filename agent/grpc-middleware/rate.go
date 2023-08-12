// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package middleware

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/tap"

	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"

	"github.com/hashicorp/consul/agent/consul/rate"
)

// ServerRateLimiterMiddleware implements a ServerInHandle function to perform
// RPC rate limiting at the cheapest possible point (before the full request has
// been decoded).
func ServerRateLimiterMiddleware(limiter rate.RequestLimitsHandler, panicHandler recovery.RecoveryHandlerFunc, logger Logger) tap.ServerInHandle {
	return func(ctx context.Context, info *tap.Info) (_ context.Context, retErr error) {
		// This function is called before unary and stream RPC interceptors, so we
		// must handle our own panics here.
		defer func() {
			if r := recover(); r != nil {
				retErr = panicHandler(r)
			}
		}()

		// Do not rate-limit the xDS service, it handles its own limiting.
		if info.FullMethodName == "/envoy.service.discovery.v3.AggregatedDiscoveryService/DeltaAggregatedResources" {
			return ctx, nil
		}

		peer, ok := peer.FromContext(ctx)
		if !ok {
			// This should never happen!
			return ctx, status.Error(codes.Internal, "gRPC rate limit middleware unable to read peer")
		}

		operationSpec, ok := rpcRateLimitSpecs[info.FullMethodName]
		if !ok {
			logger.Warn("failed to determine which rate limit to apply to RPC", "rpc", info.FullMethodName)
			return ctx, nil
		}

		err := limiter.Allow(rate.Operation{
			Name:       info.FullMethodName,
			SourceAddr: peer.Addr,
			Type:       operationSpec.Type,
			Category:   operationSpec.Category,
		})

		switch {
		case err == nil:
			return ctx, nil
		case errors.Is(err, rate.ErrRetryElsewhere):
			return ctx, status.Error(codes.ResourceExhausted, err.Error())
		case errors.Is(err, rate.ErrRetryLater):
			return ctx, status.Error(codes.Unavailable, err.Error())
		default:
			return ctx, status.Error(codes.Internal, err.Error())
		}
	}
}
