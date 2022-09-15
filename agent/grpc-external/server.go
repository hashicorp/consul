package external

import (
	"time"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
	"github.com/hashicorp/consul/tlsutil"
)

// NewServer constructs a gRPC server for the external gRPC port, to which
// handlers can be registered.
func NewServer(logger agentmiddleware.Logger, tls *tlsutil.Configurator, httpsEnabled bool) *grpc.Server {
	recoveryOpts := agentmiddleware.PanicHandlerMiddlewareOpts(logger)

	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(2048),
		middleware.WithUnaryServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.UnaryServerInterceptor(recoveryOpts...),
		),
		middleware.WithStreamServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.StreamServerInterceptor(recoveryOpts...),
		),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			// This must be less than the keealive.ClientParameters Time setting, otherwise
			// the server will disconnect the client for sending too many keepalive pings.
			// Currently the client param is set to 30s.
			MinTime: 15 * time.Second,
		}),
	}
	if tls != nil && tls.GRPCServerUseTLS(httpsEnabled) {
		creds := credentials.NewTLS(tls.IncomingGRPCConfig())
		opts = append(opts, grpc.Creds(creds))
	}
	return grpc.NewServer(opts...)
}
