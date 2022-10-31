package external

import (
	"time"

	"github.com/armon/go-metrics"
	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
)

var (
	metricsLabels = []metrics.Label{{
		Name:  "server_type",
		Value: "external",
	}}
)

// NewServer constructs a gRPC server for the external gRPC port, to which
// handlers can be registered.
func NewServer(logger agentmiddleware.Logger, metricsObj *metrics.Metrics) *grpc.Server {
	if metricsObj == nil {
		metricsObj = metrics.Default()
	}
	recoveryOpts := agentmiddleware.PanicHandlerMiddlewareOpts(logger)

	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(2048),
		grpc.MaxRecvMsgSize(50 * 1024 * 1024),
		grpc.StatsHandler(agentmiddleware.NewStatsHandler(metricsObj, metricsLabels)),
		middleware.WithUnaryServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.UnaryServerInterceptor(recoveryOpts...),
		),
		middleware.WithStreamServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.StreamServerInterceptor(recoveryOpts...),
			agentmiddleware.NewActiveStreamCounter(metricsObj, metricsLabels).Intercept,
		),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			// This must be less than the keealive.ClientParameters Time setting, otherwise
			// the server will disconnect the client for sending too many keepalive pings.
			// Currently the client param is set to 30s.
			MinTime: 15 * time.Second,
		}),
	}
	return grpc.NewServer(opts...)
}
