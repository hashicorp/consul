// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package external

import (
	"time"

	"github.com/armon/go-metrics"
	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/hashicorp/consul/agent/consul/rate"
	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
	"github.com/hashicorp/consul/tlsutil"
)

var (
	metricsLabels = []metrics.Label{{
		Name:  "server_type",
		Value: "external",
	}}
)

// NewServer constructs a gRPC server for the external gRPC port, to which
// handlers can be registered.
func NewServer(logger agentmiddleware.Logger, metricsObj *metrics.Metrics, tls *tlsutil.Configurator, limiter rate.RequestLimitsHandler) *grpc.Server {
	if metricsObj == nil {
		metricsObj = metrics.Default()
	}
	recoveryOpts := agentmiddleware.PanicHandlerMiddlewareOpts(logger)

	unaryInterceptors := []grpc.UnaryServerInterceptor{
		// Add middlware interceptors to recover in case of panics.
		recovery.UnaryServerInterceptor(recoveryOpts...),
	}
	streamInterceptors := []grpc.StreamServerInterceptor{
		// Add middlware interceptors to recover in case of panics.
		recovery.StreamServerInterceptor(recoveryOpts...),
		agentmiddleware.NewActiveStreamCounter(metricsObj, metricsLabels).Intercept,
	}

	if tls != nil {
		// Attach TLS middleware if TLS is provided.
		authInterceptor := agentmiddleware.AuthInterceptor{TLS: tls, Logger: logger}
		unaryInterceptors = append(unaryInterceptors, authInterceptor.InterceptUnary)
		streamInterceptors = append(streamInterceptors, authInterceptor.InterceptStream)
	}
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(2048),
		grpc.MaxRecvMsgSize(50 * 1024 * 1024),
		grpc.InTapHandle(agentmiddleware.ServerRateLimiterMiddleware(limiter, agentmiddleware.NewPanicHandler(logger), logger)),
		grpc.StatsHandler(agentmiddleware.NewStatsHandler(metricsObj, metricsLabels)),
		middleware.WithUnaryServerChain(unaryInterceptors...),
		middleware.WithStreamServerChain(streamInterceptors...),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			// This must be less than the keealive.ClientParameters Time setting, otherwise
			// the server will disconnect the client for sending too many keepalive pings.
			// Currently the client param is set to 30s.
			MinTime: 15 * time.Second,
		}),
	}

	if tls != nil {
		// Attach TLS credentials, if provided.
		tlsCreds := agentmiddleware.NewOptionalTransportCredentials(
			credentials.NewTLS(tls.IncomingGRPCConfig()),
			logger)
		opts = append(opts, grpc.Creds(tlsCreds))
	}
	return grpc.NewServer(opts...)
}
