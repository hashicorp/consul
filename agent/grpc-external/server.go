// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package external

import (
	"context"
	"fmt"
	"strings"
	"time"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/hashi-derek/grpc-proxy/proxy"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/consul/rate"
	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
	"github.com/hashicorp/consul/tlsutil"
)

const FORWARD_SERVICE_NAME_PREFIX = "/hashicorp.consul."

var (
	metricsLabels = []metrics.Label{{
		Name:  "server_type",
		Value: "external",
	}}
)

// NewServer constructs a gRPC server for the external gRPC port, to which
// handlers can be registered.
func NewServer(
	logger hclog.Logger,
	metricsObj *metrics.Metrics,
	tls *tlsutil.Configurator,
	limiter rate.RequestLimitsHandler,
	keepaliveParams keepalive.ServerParameters,
	serverConn *grpc.ClientConn,
) *grpc.Server {
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
		grpc.KeepaliveParams(keepaliveParams),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			// This must be less than the keealive.ClientParameters Time setting, otherwise
			// the server will disconnect the client for sending too many keepalive pings.
			// Currently the client param is set to 30s.
			MinTime: 15 * time.Second,
		}),
	}

	// forward FORWARD_SERVICE_NAME_PREFIX services from client agent to server agent
	if serverConn != nil {
		opts = append(opts, grpc.UnknownServiceHandler(proxy.TransparentHandler(makeDirector(serverConn, logger))))
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

func makeDirector(serverConn *grpc.ClientConn, logger hclog.Logger) func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
	return func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		var mdCopy metadata.MD
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			mdCopy = metadata.MD{}
		} else {
			mdCopy = md.Copy()
		}
		outCtx := metadata.NewOutgoingContext(ctx, mdCopy)

		logger.Debug("forwarding the request to the consul server", "method", fullMethodName)
		// throw unimplemented error if the method is not meant to be forwarded
		if !strings.HasPrefix(fullMethodName, FORWARD_SERVICE_NAME_PREFIX) {
			return outCtx, nil, status.Errorf(codes.Unimplemented, fmt.Sprintf("Unknown method %s", fullMethodName))
		}

		return outCtx, serverConn, nil
	}
}
