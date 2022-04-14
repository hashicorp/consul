package public

import (
	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	agentmiddleware "github.com/hashicorp/consul/agent/grpc/middleware"
	"github.com/hashicorp/consul/tlsutil"
)

// NewServer constructs a gRPC server for the public gRPC port, to which
// handlers can be registered.
func NewServer(logger agentmiddleware.Logger, tls *tlsutil.Configurator) *grpc.Server {
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
	}
	if tls != nil && tls.GRPCTLSConfigured() {
		creds := credentials.NewTLS(tls.IncomingGRPCConfig())
		opts = append(opts, grpc.Creds(creds))
	}
	return grpc.NewServer(opts...)
}
