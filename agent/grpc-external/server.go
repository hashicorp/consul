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
	"github.com/hashicorp/go-hclog"
)

// NewServer constructs a gRPC server for the external gRPC port, to which
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
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			// This must be less than the keealive.ClientParameters Time setting, otherwise
			// the server will disconnect the client for sending too many keepalive pings.
			// Currently the client param is set to 30s.
			MinTime: 15 * time.Second,
		}),
	}
	if tls != nil && tls.GRPCServerUseTLS() {
		creds := credentials.NewTLS(tls.IncomingGRPCConfig())
		opts = append(opts, grpc.Creds(creds))
	}
	return grpc.NewServer(opts...)
}

// BuildExternalGRPCServers constructs two gRPC servers for the external gRPC ports.
// This function exists because behavior for the original `ports.grpc` is dependent on
// whether the new `ports.grpc_tls` is defined. This behavior should be simplified in
// a future release so that the `ports.grpc` is always plain-text and not dependent on
// the `ports.grpc_tls` configuration.
func BuildExternalGRPCServers(grpcPort int, grpcTLSPort int, t *tlsutil.Configurator, l hclog.InterceptLogger) (grpc, grpcTLS *grpc.Server) {
	if grpcPort > 0 {
		// TODO: remove this deprecated behavior in a future version and only support plain-text for this port.
		if grpcTLSPort > 0 {
			// Use plain-text if the new grpc_tls port is configured.
			grpc = NewServer(l.Named("grpc.external"), nil)
		} else {
			// Otherwise, check TLS configuration to determine whether to encrypt (for backwards compatibility).
			grpc = NewServer(l.Named("grpc.external"), t)
			if t != nil && t.GRPCServerUseTLS() {
				l.Warn("deprecated gRPC TLS configuration detected. Consider using `ports.grpc_tls` instead")
			}
		}
	}
	if grpcTLSPort > 0 {
		if t.GRPCServerUseTLS() {
			grpcTLS = NewServer(l.Named("grpc_tls.external"), t)
		} else {
			l.Error("error starting gRPC TLS server", "error", "port is set, but invalid TLS configuration detected")
		}
	}
	return
}
